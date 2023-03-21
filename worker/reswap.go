package worker

import (
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/fifo"
	mapset "github.com/deckarep/golang-set"
)

var (
	reswapTaskQueues   = make(map[string]*fifo.Queue) // key is toChainID
	reswapTasksInQueue = mapset.NewSet()
)

// StartreswapJob reswap job
func StartReswapJob() {
	serverCfg = params.GetRouterServerConfig()
	if serverCfg == nil {
		logWorker("reswap", "stop reswap job as no router server config exist")
		return
	}
	// start producer
	go startReswapProducer()
}

func startReswapProducer() {
	logWorker("reswap", "start router reswap job")
	for {
		res, err := findRouterSwapResultToreswap()
		if err != nil {
			logWorkerError("reswap", "find out router swap error", err)
		}
		if len(res) > 0 {
			logWorker("reswap", "find out router swap", "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("reswap", "stop router swap reswap job")
				return
			}

			if !router.IsReswapSupported(swap.ToChainID) {
				continue
			}

			if reswapTasksInQueue.Contains(swap.Key) {
				logWorkerTrace("reswap", "ignore swap in queue", "key", swap.Key)
				continue
			}

			err = dispatchSwapResultToreswap(swap)
			ctx := []interface{}{"fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex}
			if err == nil {
				logWorker("reswap", "reswap router swap success", ctx...)
			} else {
				logWorkerError("reswap", "reswap router swap error", err, ctx...)
			}
		}
		if utils.IsCleanuping() {
			logWorker("reswap", "stop router swap reswap job")
			return
		}
		restInJob(restIntervalInReplaceSwapJob)
	}
}

func findRouterSwapResultToreswap() ([]*mongodb.MgoSwapResult, error) {
	septime := getSepTimeInFind(maxReplaceSwapLifetime)
	return mongodb.FindRouterSwapResultsWithStatus(mongodb.TxNeedReswap, septime)
}

func dispatchSwapResultToreswap(res *mongodb.MgoSwapResult) error {
	if !router.IsReswapSupported(res.ToChainID) {
		return tokens.ErrReswapNotSupport
	}

	chainID := res.ToChainID
	taskQueue, exist := reswapTaskQueues[chainID]
	if !exist {
		bridge := router.GetBridgeByChainID(chainID)
		if bridge == nil {
			return tokens.ErrNoBridgeForChainID
		}
		// init reswap task queue and start consumer routine
		taskQueue = fifo.NewQueue()
		reswapTaskQueues[chainID] = taskQueue
		mongodb.MgoWaitGroup.Add(1)
		go startReswapConsumer(chainID)
	}

	logWorker("reswap", "dispatch reswap router task", "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "txid", res.TxID, "logIndex", res.LogIndex, "value", res.SwapValue, "swapNonce", res.SwapNonce, "queue", taskQueue.Len())

	taskQueue.Add(res)
	reswapTasksInQueue.Add(res.Key)

	return nil
}

func startReswapConsumer(chainID string) {
	defer mongodb.MgoWaitGroup.Done()
	logWorker("reswap", "start reswap swap task", "chainID", chainID)

	taskQueue, exist := reswapTaskQueues[chainID]
	if !exist {
		log.Fatal("no reswap task queue", "chainID", chainID)
	}

	i := 0
	for {
		if utils.IsCleanuping() {
			logWorker("doreswap", "stop reswap swap task", "chainID", chainID)
			return
		}

		if i%10 == 0 && taskQueue.Len() > 0 {
			logWorker("doreswap", "tasks in reswap queue", "chainID", chainID, "count", taskQueue.Len())
		}
		i++

		front := taskQueue.Next()
		if front == nil {
			sleepSeconds(3)
			continue
		}

		swap := front.(*mongodb.MgoSwapResult)

		if swap.ToChainID != chainID {
			logWorkerWarn("doreswap", "ignore reswap task as toChainID mismatch", "want", chainID, "swap", swap)
			continue
		}

		ctx := []interface{}{"fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex}
		err := reswapRouterSwap(swap, false)
		if err == nil {
			logWorker("doreswap", "reswap router swap success", ctx...)
		} else {
			logWorkerError("doreswap", "reswap router swap failed", err, ctx...)
		}

		reswapTasksInQueue.Remove(swap.Key)
	}
}

// reswapRouterSwap api
func reswapRouterSwap(res *mongodb.MgoSwapResult, isManual bool) error {
	if !router.IsReswapSupported(res.ToChainID) {
		return tokens.ErrReswapNotSupport
	}
	swap, err := verifyReswapSwap(res, isManual)
	if err != nil {
		return err
	}
	routerMPC, err := router.GetRouterMPC(swap.GetTokenID(), res.ToChainID)
	if err != nil {
		return err
	}
	if !common.IsEqualIgnoreCase(res.MPC, routerMPC) {
		return tokens.ErrSenderMismatch
	}

	logWorker("reswapSwap", "process task", "swap", res)

	_ = updateSwapTimestamp(res.FromChainID, res.TxID, res.LogIndex)
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	biFromChainID, biToChainID, biValue, err := getFromToChainIDAndValue(res.FromChainID, res.ToChainID, res.Value)
	if err != nil {
		return err
	}
	txid := res.TxID
	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			Identifier:  params.GetIdentifier(),
			SwapID:      txid,
			SwapType:    tokens.SwapType(res.SwapType),
			Bind:        res.Bind,
			LogIndex:    res.LogIndex,
			FromChainID: biFromChainID,
			ToChainID:   biToChainID,
		},
		From:        res.MPC,
		OriginFrom:  swap.From,
		OriginTxTo:  swap.TxTo,
		OriginValue: biValue,
		Extra:       &tokens.AllExtras{},
	}
	args.SwapInfo, err = mongodb.ConvertFromSwapInfo(&swap.SwapInfo)
	if err != nil {
		return err
	}
	rawTx, err := resBridge.BuildRawTransaction(args)
	if err != nil {
		logWorkerError("reswapSwap", "build tx failed", err, "chainID", res.ToChainID, "txid", txid, "logIndex", res.LogIndex)
		return err
	}
	go signAndSendReswapTx(resBridge, rawTx, args, res)
	return nil
}

func signAndSendReswapTx(resBridge tokens.IBridge, rawTx interface{}, args *tokens.BuildTxArgs, res *mongodb.MgoSwapResult) {
	signedTx, txHash, err := resBridge.MPCSignTransaction(rawTx, args)
	if err != nil {
		logWorkerError("reswapSwap", "mpc sign tx failed", err, "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "txid", res.TxID, "nonce", res.SwapNonce, "logIndex", res.LogIndex)
		if errors.Is(err, mpc.ErrGetSignStatusHasDisagree) {
			reverifySwap(args)
		}
		return
	}

	fromChainID := res.FromChainID
	txid := res.TxID
	logIndex := res.LogIndex

	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
	disagreeRecords.Delete(cacheKey)

	// update database before sending transaction
	addSwapHistory(fromChainID, txid, logIndex, txHash)

	err = mongodb.UpdateRouterOldSwapTxs(fromChainID, txid, logIndex, txHash)
	if err != nil {
		return
	}

	matchTx := &MatchTx{
		SwapTx:    txHash,
		SwapNonce: 0,
		SwapValue: args.SwapValue.String(),
		MPC:       args.From,
		TTL:       *args.Extra.TTL,
	}

	err = updateRouterSwapResult(fromChainID, txid, logIndex, matchTx)
	if err != nil {
		logWorkerError("doSwap", "update router swap result failed", err, "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex, "ttl", *args.Extra.TTL)
		return
	}

	sentTxHash, err := sendSignedTransaction(resBridge, signedTx, args)
	if err == nil && txHash != sentTxHash {
		logWorkerError("reswapSwap", "send tx success but with different hash", errSendTxWithDiffHash,
			"fromChainID", fromChainID, "toChainID", res.ToChainID, "txid", txid, "nonce", res.SwapNonce,
			"logIndex", logIndex, "txHash", txHash, "sentTxHash", sentTxHash)
		_ = mongodb.UpdateRouterOldSwapTxs(fromChainID, txid, logIndex, sentTxHash)
	}

}

func verifyReswapSwap(res *mongodb.MgoSwapResult, isManual bool) (*mongodb.MgoSwap, error) {
	fromChainID, txid, logIndex := res.FromChainID, res.TxID, res.LogIndex
	swap, err := mongodb.FindRouterSwap(fromChainID, txid, logIndex)
	if err != nil {
		return nil, err
	}
	if isBlacked(swap) {
		logWorkerWarn("reswap", "swap is in black list", "txid", res.TxID, "logIndex", res.LogIndex, "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "token", res.GetToken(), "tokenID", res.GetTokenID())
		err = tokens.ErrSwapInBlacklist
		_ = mongodb.UpdateRouterSwapStatus(res.FromChainID, res.TxID, res.LogIndex, mongodb.SwapInBlacklist, now(), err.Error())
		_ = mongodb.UpdateRouterSwapResultStatus(res.FromChainID, res.TxID, res.LogIndex, mongodb.SwapInBlacklist, now(), err.Error())
		return nil, err
	}
	if swap.Status != mongodb.TxProcessed {
		return nil, fmt.Errorf("cannot reswap swap with status not equal to 'TxProcessed'")
	}
	if res.Status != mongodb.TxNeedReswap {
		return nil, errors.New("swap result status is not 'TxNeedReswap'")
	}
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	if resBridge == nil {
		return nil, tokens.ErrNoBridgeForChainID
	}
	txStatus, _ := resBridge.GetTransactionStatus(res.SwapTx)
	if txStatus != nil && txStatus.BlockHeight > 0 {
		return nil, errors.New("swap tx existed ")
	}
	return swap, nil
}

func reswapIfTimeout(bridge tokens.IBridge, res *mongodb.MgoSwapResult) error {
	b, ok := bridge.(tokens.ReSwapable)
	if !ok {
		return nil
	}
	threshold, err := b.GetCurrentThreshold()
	if err != nil {
		return err
	}
	if b.IsTxTimeout(&res.TTL, threshold) {
		err := mongodb.UpdateRouterSwapResultStatus(res.FromChainID, res.TxID, res.LogIndex, mongodb.TxNeedReswap, now(), fmt.Sprintf("ttl:%d current:%d", *threshold, res.TTL))
		return err
	}
	return errors.New("reswap check tx not Timeout yet")
}

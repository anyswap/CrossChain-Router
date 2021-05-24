package worker

import (
	"container/ring"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/mongodb"
	"github.com/anyswap/CrossChain-Router/params"
	"github.com/anyswap/CrossChain-Router/router"
	"github.com/anyswap/CrossChain-Router/tokens"
)

var (
	swapRing        *ring.Ring
	swapRingLock    sync.RWMutex
	swapRingMaxSize = 1000

	swapChanSize          = 10
	routerSwapTaskChanMap = make(map[string]chan *tokens.BuildTxArgs) // key is chainID

	errAlreadySwapped = errors.New("already swapped")
)

// StartSwapJob swap job
func StartSwapJob() {
	for _, bridge := range router.RouterBridges {
		chainID := bridge.GetChainConfig().ChainID
		if _, exist := routerSwapTaskChanMap[chainID]; !exist {
			routerSwapTaskChanMap[chainID] = make(chan *tokens.BuildTxArgs, swapChanSize)
			go processSwapTask(routerSwapTaskChanMap[chainID])
		}

		go startRouterSwapJob(chainID)
	}
}

func startRouterSwapJob(chainID string) {
	logWorker("swap", "start router swap job")
	for {
		res, err := findRouterSwapToSwap(chainID)
		if err != nil {
			logWorkerError("swap", "find out router swap error", err)
		}
		if len(res) > 0 {
			logWorker("swap", "find out router swap", "count", len(res))
		}
		for _, swap := range res {
			err = processRouterSwap(swap)
			switch {
			case err == nil,
				errors.Is(err, errAlreadySwapped):
			default:
				logWorkerError("swap", "process router swap error", err, "chainID", chainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		restInJob(restIntervalInDoSwapJob)
	}
}

func findRouterSwapToSwap(chainID string) ([]*mongodb.MgoSwap, error) {
	status := mongodb.TxNotSwapped
	septime := getSepTimeInFind(maxDoSwapLifetime)
	return mongodb.FindRouterSwapsWithChainIDAndStatus(chainID, status, septime)
}

func processRouterSwap(swap *mongodb.MgoSwap) (err error) {
	fromChainID := swap.FromChainID
	toChainID := swap.ToChainID
	txid := swap.TxID
	logIndex := swap.LogIndex
	bind := swap.Bind

	if params.IsSwapInBlacklist(fromChainID, toChainID, swap.GetTokenID()) {
		logWorkerTrace("swap", "swap is in black list", "txid", txid, "logIndex", logIndex,
			"fromChainID", fromChainID, "toChainID", toChainID, "token", swap.Token, "tokenID", swap.GetTokenID())
		err = tokens.ErrSwapInBlacklist
		_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.SwapInBlacklist, now(), err.Error())
		return nil
	}

	res, err := mongodb.FindRouterSwapResult(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}

	logWorker("swap", "start process router swap", "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex, "status", swap.Status, "value", res.Value)

	dstBridge := router.GetBridgeByChainID(toChainID)
	if dstBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	err = preventReswap(res)
	if err != nil {
		return err
	}

	biFromChainID, biToChainID, biValue, err := getFromToChainIDAndValue(fromChainID, toChainID, res.Value)
	if err != nil {
		return err
	}

	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			Identifier:  params.GetIdentifier(),
			SwapID:      txid,
			SwapType:    tokens.SwapType(swap.SwapType),
			Bind:        bind,
			LogIndex:    swap.LogIndex,
			FromChainID: biFromChainID,
			ToChainID:   biToChainID,
		},
		From:        dstBridge.GetChainConfig().GetRouterMPC(),
		OriginValue: biValue,
	}
	args.SwapInfo, err = mongodb.ConvertFromSwapInfo(&swap.SwapInfo)
	if err != nil {
		return err
	}

	return dispatchSwapTask(args)
}

func getFromToChainIDAndValue(fromChainIDStr, toChainIDStr, valueStr string) (fromChainID, toChainID, value *big.Int, err error) {
	fromChainID, err = common.GetBigIntFromStr(fromChainIDStr)
	if err != nil {
		err = fmt.Errorf("wrong fromChainID %v", fromChainIDStr)
		return
	}
	toChainID, err = common.GetBigIntFromStr(toChainIDStr)
	if err != nil {
		err = fmt.Errorf("wrong toChainID %v", toChainIDStr)
		return
	}
	value, err = common.GetBigIntFromStr(valueStr)
	if err != nil {
		err = fmt.Errorf("wrong value %v", valueStr)
		return
	}
	return
}

func preventReswap(res *mongodb.MgoSwapResult) (err error) {
	err = processNonEmptySwapResult(res)
	if err != nil {
		return err
	}
	return processHistory(res)
}

func processNonEmptySwapResult(res *mongodb.MgoSwapResult) error {
	if res.SwapNonce > 0 ||
		res.Status != mongodb.MatchTxEmpty ||
		res.SwapTx != "" ||
		res.SwapHeight != 0 ||
		len(res.OldSwapTxs) > 0 {
		_ = mongodb.UpdateRouterSwapStatus(res.FromChainID, res.TxID, res.LogIndex, mongodb.TxProcessed, now(), "")
		return errAlreadySwapped
	}
	return nil
}

func processHistory(res *mongodb.MgoSwapResult) error {
	chainID := res.FromChainID
	txid := res.TxID
	logIndex := res.LogIndex
	history := getSwapHistory(chainID, txid, logIndex)
	if history == nil {
		return nil
	}
	if res.Status == mongodb.MatchTxFailed || res.Status == mongodb.MatchTxEmpty {
		history.txid = "" // mark ineffective
		return nil
	}
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	if _, err := resBridge.GetTransaction(history.matchTx); err == nil {
		_ = mongodb.UpdateRouterSwapStatus(chainID, txid, logIndex, mongodb.TxProcessed, now(), "")
		logWorker("swap", "ignore swapped router swap", "chainID", chainID, "txid", txid, "matchTx", history.matchTx)
		return errAlreadySwapped
	}
	return nil
}

func dispatchSwapTask(args *tokens.BuildTxArgs) error {
	switch args.SwapType {
	case tokens.RouterSwapType, tokens.AnyCallSwapType:
		swapChan, exist := routerSwapTaskChanMap[args.FromChainID.String()]
		if !exist {
			return fmt.Errorf("no swapout task channel for chainID '%v'", args.FromChainID)
		}
		swapChan <- args
	default:
		return fmt.Errorf("wrong swap type '%v'", args.SwapType.String())
	}
	logWorker("doSwap", "dispatch router swap task", "chainID", args.FromChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "value", args.OriginValue)
	return nil
}

func processSwapTask(swapChan <-chan *tokens.BuildTxArgs) {
	for {
		args := <-swapChan
		err := doSwap(args)
		switch {
		case err == nil,
			errors.Is(err, errAlreadySwapped),
			errors.Is(err, tokens.ErrNoBridgeForChainID):
		default:
			logWorkerError("doSwap", "process router swap failed", err, "args", args)
		}
	}
}

func doSwap(args *tokens.BuildTxArgs) (err error) {
	fromChainID := args.FromChainID.String()
	toChainID := args.ToChainID.String()
	txid := args.SwapID
	logIndex := args.LogIndex
	originValue := args.OriginValue

	res, err := mongodb.FindRouterSwapResult(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}
	err = preventReswap(res)
	if err != nil {
		return err
	}

	logWorker("doSwap", "start to process", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "value", originValue)

	resBridge := router.GetBridgeByChainID(toChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	rawTx, err := resBridge.BuildRawTransaction(args)
	if err != nil {
		logWorkerError("doSwap", "build tx failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		if errors.Is(err, tokens.ErrEstimateGasFailed) {
			_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.EstimateGasFailed, now(), err.Error())
			_ = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, mongodb.EstimateGasFailed, now(), err.Error())
		}
		return err
	}

	signedTx, txHash, err := resBridge.MPCSignTransaction(rawTx, args.GetExtraArgs())
	if err != nil {
		logWorkerError("doSwap", "sign tx failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		return err
	}

	swapTxNonce := args.GetTxNonce()

	var oldSwapTxs []string
	if len(res.OldSwapTxs) > 0 {
		var existsInOld bool
		for _, oldSwapTx := range res.OldSwapTxs {
			if oldSwapTx == txHash {
				existsInOld = true
				break
			}
		}
		if !existsInOld {
			oldSwapTxs = res.OldSwapTxs
			oldSwapTxs = append(oldSwapTxs, txHash)
		}
	} else if res.SwapTx != "" && txHash != res.SwapTx {
		oldSwapTxs = []string{res.SwapTx, txHash}
	}

	// update database before sending transaction
	addSwapHistory(fromChainID, txid, logIndex, args.SwapValue, txHash, swapTxNonce)
	matchTx := &MatchTx{
		SwapTx:     txHash,
		OldSwapTxs: oldSwapTxs,
		SwapValue:  args.SwapValue.String(),
		SwapNonce:  swapTxNonce,
	}
	err = updateRouterSwapResult(fromChainID, txid, logIndex, matchTx)
	if err != nil {
		logWorkerError("doSwap", "update router swap result failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		return err
	}

	err = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxProcessed, now(), "")
	if err != nil {
		logWorkerError("doSwap", "update router swap status failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		return err
	}

	return sendSignedTransaction(resBridge, signedTx, args, false)
}

type swapInfo struct {
	chainID   string
	txid      string
	logIndex  int
	swapValue *big.Int
	matchTx   string
	nonce     uint64
}

func addSwapHistory(chainID, txid string, logIndex int, swapValue *big.Int, matchTx string, nonce uint64) {
	// Create the new item as its own ring
	item := ring.New(1)
	item.Value = &swapInfo{
		chainID:   chainID,
		txid:      txid,
		logIndex:  logIndex,
		swapValue: swapValue,
		matchTx:   matchTx,
		nonce:     nonce,
	}

	swapRingLock.Lock()
	defer swapRingLock.Unlock()

	if swapRing == nil {
		swapRing = item
	} else {
		if swapRing.Len() == swapRingMaxSize {
			swapRing = swapRing.Move(-1)
			swapRing.Unlink(1)
			swapRing = swapRing.Move(1)
		}
		swapRing.Move(-1).Link(item)
	}
}

func getSwapHistory(chainID, txid string, logIndex int) *swapInfo {
	swapRingLock.RLock()
	defer swapRingLock.RUnlock()

	if swapRing == nil {
		return nil
	}

	r := swapRing
	for i := 0; i < r.Len(); i++ {
		item := r.Value.(*swapInfo)
		if item.txid == txid && item.chainID == chainID && item.logIndex == logIndex {
			return item
		}
		r = r.Prev()
	}

	return nil
}

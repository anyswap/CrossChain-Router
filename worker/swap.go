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
			switch err {
			case nil, errAlreadySwapped:
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

	if params.IsSwapInBlacklist(fromChainID, toChainID, swap.TokenID) {
		logWorkerTrace("swap", "swap is in black list", "txid", txid, "logIndex", logIndex,
			"fromChainID", fromChainID, "toChainID", toChainID, "token", swap.Token, "tokenID", swap.TokenID)
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
	if err != nil {
		return err
	}

	err = preventReswap(res)
	if err != nil {
		return err
	}

	value, err := common.GetBigIntFromStr(res.Value)
	if err != nil {
		return fmt.Errorf("wrong value %v", res.Value)
	}

	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			Identifier: params.GetIdentifier(),
			SwapID:     txid,
			SwapType:   tokens.SwapType(swap.SwapType),
			Bind:       bind,
		},
		From:        dstBridge.GetChainConfig().GetRouterMPC(),
		OriginValue: value,
	}
	args.RouterSwapInfo, err = getRouterSwapInfoFromSwap(swap)
	if err != nil {
		return err
	}

	return dispatchSwapTask(args)
}

func getRouterSwapInfoFromSwap(swap *mongodb.MgoSwap) (*tokens.RouterSwapInfo, error) {
	var amountOutMin *big.Int
	var err error
	if len(swap.Path) > 0 {
		amountOutMin, err = common.GetBigIntFromStr(swap.AmountOutMin)
		if err != nil {
			return nil, fmt.Errorf("wrong amountOutMin %v", swap.AmountOutMin)
		}
	}
	fromChainID, err := common.GetBigIntFromStr(swap.FromChainID)
	if err != nil {
		return nil, fmt.Errorf("wrong fromChainID %v", swap.FromChainID)
	}
	toChainID, err := common.GetBigIntFromStr(swap.ToChainID)
	if err != nil {
		return nil, fmt.Errorf("wrong toChainID %v", swap.ToChainID)
	}
	return &tokens.RouterSwapInfo{
		ForNative:     swap.ForNative,
		ForUnderlying: swap.ForUnderlying,
		Token:         swap.Token,
		TokenID:       swap.TokenID,
		Path:          swap.Path,
		AmountOutMin:  amountOutMin,
		FromChainID:   fromChainID,
		ToChainID:     toChainID,
		LogIndex:      swap.LogIndex,
	}, nil
}

func preventReswap(res *mongodb.MgoSwapResult) (err error) {
	err = processNonEmptySwapResult(res)
	if err != nil {
		return err
	}
	return processHistory(res)
}

func processNonEmptySwapResult(res *mongodb.MgoSwapResult) error {
	if res.SwapTx == "" {
		return nil
	}
	chainID := res.FromChainID
	txid := res.TxID
	logIndex := res.LogIndex
	_ = mongodb.UpdateRouterSwapStatus(chainID, txid, logIndex, mongodb.TxProcessed, now(), "")
	if res.Status != mongodb.MatchTxEmpty {
		return errAlreadySwapped
	}
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	if _, err := resBridge.GetTransaction(res.SwapTx); err == nil {
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
	if res.Status == mongodb.MatchTxFailed {
		history.txid = "" // mark ineffective
		return nil
	}
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	if _, err := resBridge.GetTransaction(history.matchTx); err == nil {
		matchTx := &MatchTx{
			SwapTx:    history.matchTx,
			SwapValue: history.swapValue.String(),
			SwapNonce: history.nonce,
		}
		_ = updateRouterSwapResult(chainID, txid, logIndex, matchTx)
		logWorker("swap", "ignore swapped router swap", "chainID", chainID, "txid", txid, "matchTx", history.matchTx)
		return errAlreadySwapped
	}
	return nil
}

func dispatchSwapTask(args *tokens.BuildTxArgs) error {
	switch args.SwapType {
	case tokens.RouterSwapType:
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
		switch err {
		case nil, errAlreadySwapped:
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
		if err == tokens.ErrEstimateGasFailed {
			_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.EstimateGasFailed, now(), err.Error())
			_ = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, mongodb.EstimateGasFailed, now(), err.Error())
		}
		return err
	}

	signedTx, txHash, err := mpcSignTransaction(resBridge, rawTx, args.GetExtraArgs())
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

package worker

import (
	"container/ring"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	mapset "github.com/deckarep/golang-set"
)

var (
	swapRing        *ring.Ring
	swapRingLock    sync.RWMutex
	swapRingMaxSize = 1000

	cachedSwapTasks    = mapset.NewSet()
	maxCachedSwapTasks = 1000

	swapChanSize          = 10
	routerSwapTaskChanMap = make(map[string]chan *tokens.BuildTxArgs) // key is chainID

	errAlreadySwapped     = errors.New("already swapped")
	errSendTxWithDiffHash = errors.New("send tx with different hash")
)

// StartSwapJob swap job
func StartSwapJob() {
	for _, bridge := range router.RouterBridges {
		chainID := bridge.GetChainConfig().ChainID
		if _, exist := routerSwapTaskChanMap[chainID]; !exist {
			routerSwapTaskChanMap[chainID] = make(chan *tokens.BuildTxArgs, swapChanSize)
			utils.TopWaitGroup.Add(1)
			go processSwapTask(chainID, routerSwapTaskChanMap[chainID])
		}

		mongodb.MgoWaitGroup.Add(1)
		go startRouterSwapJob(chainID)
	}
}

func startRouterSwapJob(chainID string) {
	defer mongodb.MgoWaitGroup.Done()
	logWorker("swap", "start router swap job", "chainID", chainID)
	for {
		res, err := findRouterSwapToSwap(chainID)
		if err != nil {
			logWorkerError("swap", "find out router swap error", err)
		}
		if len(res) > 0 {
			logWorker("swap", "find out router swap", "chainID", chainID, "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("swap", "stop router swap job", "chainID", chainID)
				return
			}
			err = processRouterSwap(swap)
			switch {
			case err == nil,
				errors.Is(err, errAlreadySwapped):
			default:
				logWorkerError("swap", "process router swap error", err, "chainID", chainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		if utils.IsCleanuping() {
			logWorker("swap", "stop router swap job", "chainID", chainID)
			return
		}
		restInJob(restIntervalInDoSwapJob)
	}
}

func findRouterSwapToSwap(chainID string) ([]*mongodb.MgoSwap, error) {
	status := mongodb.TxNotSwapped
	septime := getSepTimeInFind(maxDoSwapLifetime)
	return mongodb.FindRouterSwapsWithChainIDAndStatus(chainID, status, septime)
}

func getSwapCacheKey(fromChainID, txid string, logIndex int) string {
	return strings.ToLower(fmt.Sprintf("%s:%s:%d", fromChainID, txid, logIndex))
}

func processRouterSwap(swap *mongodb.MgoSwap) (err error) {
	fromChainID := swap.FromChainID
	toChainID := swap.ToChainID
	txid := swap.TxID
	logIndex := swap.LogIndex
	bind := swap.Bind

	cacheKey := getSwapCacheKey(fromChainID, txid, logIndex)
	if cachedSwapTasks.Contains(cacheKey) {
		return errAlreadySwapped
	}

	if params.IsSwapInBlacklist(fromChainID, toChainID, swap.GetTokenID()) {
		logWorkerTrace("swap", "swap is in black list", "txid", txid, "logIndex", logIndex,
			"fromChainID", fromChainID, "toChainID", toChainID, "token", swap.GetToken(), "tokenID", swap.GetTokenID())
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
			Reswapping:  res.Status == mongodb.Reswapping,
		},
		From:        dstBridge.GetChainConfig().GetRouterMPC(),
		OriginFrom:  swap.From,
		OriginTxTo:  swap.TxTo,
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
		!(res.Status == mongodb.MatchTxEmpty || res.Status == mongodb.Reswapping) ||
		res.SwapTx != "" ||
		res.SwapHeight != 0 ||
		len(res.OldSwapTxs) > 0 {
		_ = mongodb.UpdateRouterSwapStatus(res.FromChainID, res.TxID, res.LogIndex, mongodb.TxProcessed, now(), "")
		return errAlreadySwapped
	}
	return nil
}

func processHistory(res *mongodb.MgoSwapResult) error {
	if (res.Status == mongodb.MatchTxEmpty || res.Status == mongodb.Reswapping) && res.SwapNonce == 0 {
		return nil
	}
	chainID := res.FromChainID
	txid := res.TxID
	logIndex := res.LogIndex
	history := getSwapHistory(chainID, txid, logIndex)
	if history == nil {
		return nil
	}
	_ = mongodb.UpdateRouterSwapStatus(chainID, txid, logIndex, mongodb.TxProcessed, now(), "")
	logWorker("swap", "ignore swapped router swap", "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "txid", txid, "logIndex", logIndex, "matchTx", history.matchTx)
	return errAlreadySwapped
}

func dispatchSwapTask(args *tokens.BuildTxArgs) error {
	if !args.SwapType.IsValidType() {
		return fmt.Errorf("unknown router swap type %d", args.SwapType)
	}
	toChainID := args.ToChainID.String()
	swapChan, exist := routerSwapTaskChanMap[toChainID]
	if !exist {
		return fmt.Errorf("no swapout task channel for chainID '%v'", args.ToChainID)
	}
	swapChan <- args

	logWorker("doSwap", "dispatch router swap task", "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "value", args.OriginValue, "swapNonce", args.GetTxNonce())
	return nil
}

func processSwapTask(chainID string, swapChan <-chan *tokens.BuildTxArgs) {
	defer utils.TopWaitGroup.Done()
	for {
		select {
		case <-utils.CleanupChan:
			logWorker("doSwap", "stop process swap task", "chainID", chainID)
			return
		case args := <-swapChan:
			if args.ToChainID.String() != chainID {
				logWorkerWarn("doSwap", "ignore swap task as toChainID mismatch", "want", chainID, "args", args)
				continue
			}
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
}

func checkAndUpdateProcessSwapTaskCache(key string) error {
	if cachedSwapTasks.Contains(key) {
		return errAlreadySwapped
	}
	if cachedSwapTasks.Cardinality() >= maxCachedSwapTasks {
		cachedSwapTasks.Pop()
	}
	cachedSwapTasks.Add(key)
	return nil
}

func doSwap(args *tokens.BuildTxArgs) (err error) {
	fromChainID := args.FromChainID.String()
	toChainID := args.ToChainID.String()
	txid := args.SwapID
	logIndex := args.LogIndex
	originValue := args.OriginValue

	cacheKey := getSwapCacheKey(fromChainID, txid, logIndex)
	err = checkAndUpdateProcessSwapTaskCache(cacheKey)
	if err != nil {
		return err
	}
	logWorker("doSwap", "add swap cache", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "value", originValue)
	isCachedSwapProcessed := false
	defer func() {
		if !isCachedSwapProcessed {
			logWorkerError("doSwap", "delete swap cache", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "value", originValue)
			cachedSwapTasks.Remove(cacheKey)
		}
	}()

	logWorker("doSwap", "start to process", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "value", originValue)

	resBridge := router.GetBridgeByChainID(toChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	rawTx, err := resBridge.BuildRawTransaction(args)
	if err != nil {
		logWorkerError("doSwap", "build tx failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		return err
	}

	signedTx, txHash, err := resBridge.MPCSignTransaction(rawTx, args.GetExtraArgs())
	if err != nil {
		logWorkerError("doSwap", "sign tx failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		return err
	}

	// recheck reswap before update db
	res, err := mongodb.FindRouterSwapResult(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}
	err = preventReswap(res)
	if err != nil {
		return err
	}

	swapTxNonce := args.GetTxNonce()

	// update database before sending transaction
	addSwapHistory(fromChainID, txid, logIndex, txHash)
	matchTx := &MatchTx{
		SwapTx:    txHash,
		SwapValue: args.SwapValue.String(),
		SwapNonce: swapTxNonce,
		MPC:       args.From,
	}
	err = updateRouterSwapResult(fromChainID, txid, logIndex, matchTx)
	if err != nil {
		logWorkerError("doSwap", "update router swap result failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "swapNonce", swapTxNonce)
		return err
	}
	isCachedSwapProcessed = true

	err = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxProcessed, now(), "")
	if err != nil {
		logWorkerError("doSwap", "update router swap status failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		return err
	}

	sentTxHash, err := sendSignedTransaction(resBridge, signedTx, args)
	if err == nil {
		if txHash != sentTxHash {
			logWorkerError("doSwap", "send tx success but with different hash", errSendTxWithDiffHash, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "txHash", txHash, "sentTxHash", sentTxHash, "swapNonce", swapTxNonce)
			_ = replaceSwapResult(res, sentTxHash)
		}
	}
	return err
}

// DeleteCachedSwap delete cached swap
func DeleteCachedSwap(fromChainID, txid string, logIndex int) {
	cacheKey := getSwapCacheKey(fromChainID, txid, logIndex)
	cachedSwapTasks.Remove(cacheKey)
}

type swapInfo struct {
	chainID  string
	txid     string
	logIndex int
	matchTx  string
}

func addSwapHistory(chainID, txid string, logIndex int, matchTx string) {
	// Create the new item as its own ring
	item := ring.New(1)
	item.Value = &swapInfo{
		chainID:  chainID,
		txid:     txid,
		logIndex: logIndex,
		matchTx:  matchTx,
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

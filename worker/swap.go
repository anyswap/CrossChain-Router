package worker

import (
	"container/ring"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
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

	swapChanSize          = 100
	routerSwapTaskChanMap = make(map[string]chan *tokens.BuildTxArgs) // key is chainID
	routerSwapTasksInChan = mapset.NewSet()

	errAlreadySwapped     = errors.New("already swapped")
	errSendTxWithDiffHash = errors.New("send tx with different hash")
	errSwapChannelIsFull  = errors.New("swap task channel is full")
)

// StartSwapJob swap job
func StartSwapJob() {
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)

		if _, exist := routerSwapTaskChanMap[chainID]; !exist {
			routerSwapTaskChanMap[chainID] = make(chan *tokens.BuildTxArgs, swapChanSize)
			utils.TopWaitGroup.Add(1)
			go processSwapTask(chainID, routerSwapTaskChanMap[chainID])
		}

		mongodb.MgoWaitGroup.Add(1)
		go startRouterSwapJob(chainID)

		return true
	})
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
			case err == nil:
			case errors.Is(err, errAlreadySwapped),
				errors.Is(err, errSwapChannelIsFull):
				logWorkerTrace("swap", "process router swap error", err, "chainID", chainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
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

func processRouterSwap(swap *mongodb.MgoSwap) (err error) {
	if router.IsChainIDPaused(swap.FromChainID) || router.IsChainIDPaused(swap.ToChainID) {
		return nil
	}

	fromChainID := swap.FromChainID
	toChainID := swap.ToChainID
	txid := swap.TxID
	logIndex := swap.LogIndex
	bind := swap.Bind

	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
	if routerSwapTasksInChan.Contains(cacheKey) {
		return nil
	}
	if cachedSwapTasks.Contains(cacheKey) {
		return errAlreadySwapped
	}

	if isBlacked(swap) {
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

	routerMPC, err := router.GetRouterMPC(swap.GetTokenID(), toChainID)
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
		From:        routerMPC,
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

	ctx := []interface{}{
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "value", args.OriginValue, "swapNonce", args.GetTxNonce(),
	}

	select {
	case swapChan <- args:
		logWorker("doSwap", "dispatch router swap task", ctx...)
		cacheKey := mongodb.GetRouterSwapKey(args.FromChainID.String(), args.SwapID, args.LogIndex)
		routerSwapTasksInChan.Add(cacheKey)
	default:
		logWorkerWarn("doSwap", "swap task channel is full", ctx...)
		return errSwapChannelIsFull
	}

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
			case err == nil:
				logWorker("doSwap", "process router swap success", "args", args)
			case errors.Is(err, errAlreadySwapped),
				errors.Is(err, tokens.ErrNoBridgeForChainID):
				logWorkerTrace("doSwap", "process router swap failed", err, "args", args)
			default:
				logWorkerError("doSwap", "process router swap failed", err, "args", args)
			}
			cacheKey := mongodb.GetRouterSwapKey(args.FromChainID.String(), args.SwapID, args.LogIndex)
			routerSwapTasksInChan.Remove(cacheKey)
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

//nolint:funlen,gocyclo // ok
func doSwap(args *tokens.BuildTxArgs) (err error) {
	if params.IsParallelSwapEnabled() {
		return doSwapParallel(args)
	}

	fromChainID := args.FromChainID.String()
	toChainID := args.ToChainID.String()
	txid := args.SwapID
	logIndex := args.LogIndex
	originValue := args.OriginValue

	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
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
	swapTxNonce := args.GetTxNonce() // assign after build tx
	logWorker("doSwap", "build tx success", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "swapNonce", swapTxNonce)

	signedTx, txHash, err := resBridge.MPCSignTransaction(rawTx, args)
	if err != nil {
		logWorkerError("doSwap", "sign tx failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		if errors.Is(err, mpc.ErrGetSignStatusHasDisagree) {
			reverifySwap(args)
		}
		return err
	}
	logWorker("doSwap", "sign tx success", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "txHash", txHash, "swapNonce", swapTxNonce)

	// recheck reswap before update db
	res, err := mongodb.FindRouterSwapResult(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}
	err = preventReswap(res)
	if err != nil {
		return err
	}

	// update database before sending transaction
	addSwapHistory(fromChainID, txid, logIndex, txHash)
	matchTx := &MatchTx{
		SwapTx:    txHash,
		SwapNonce: swapTxNonce,
		MPC:       args.From,
	}
	if args.SwapValue != nil {
		matchTx.SwapValue = args.SwapValue.String()
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
	if err == nil && txHash != sentTxHash {
		logWorkerError("doSwap", "send tx success but with different hash", errSendTxWithDiffHash,
			"fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex,
			"txHash", txHash, "sentTxHash", sentTxHash, "swapNonce", swapTxNonce)
		_ = mongodb.UpdateRouterOldSwapTxs(fromChainID, txid, logIndex, sentTxHash)
	} else if err == nil {
		logWorker("doSwap", "send tx success",
			"fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex,
			"txHash", txHash, "swapNonce", swapTxNonce)
	}
	logWorker("doSwap", "finish to process", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "value", originValue)
	return err
}

func doSwapParallel(args *tokens.BuildTxArgs) (err error) {
	fromChainID := args.FromChainID.String()
	toChainID := args.ToChainID.String()
	txid := args.SwapID
	logIndex := args.LogIndex
	originValue := args.OriginValue

	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
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

	isCachedSwapProcessed = true
	go func() {
		_ = signAndSendTx(rawTx, args)
	}()
	return nil
}

func signAndSendTx(rawTx interface{}, args *tokens.BuildTxArgs) error {
	fromChainID := args.FromChainID.String()
	toChainID := args.ToChainID.String()
	txid := args.SwapID
	logIndex := args.LogIndex
	swapTxNonce := args.GetTxNonce()
	resBridge := router.GetBridgeByChainID(toChainID)

	signedTx, txHash, err := resBridge.MPCSignTransaction(rawTx, args)
	if err != nil {
		logWorkerError("doSwap", "sign tx failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "swapNonce", swapTxNonce)
		if errors.Is(err, mpc.ErrGetSignStatusHasDisagree) {
			reverifySwap(args)
		}
		return err
	}

	// update database before sending transaction
	addSwapHistory(fromChainID, txid, logIndex, txHash)
	_ = updateSwapTx(fromChainID, txid, logIndex, txHash)

	sentTxHash, err := sendSignedTransaction(resBridge, signedTx, args)
	if err == nil && txHash != sentTxHash {
		logWorkerError("doSwap", "send tx success but with different hash", errSendTxWithDiffHash,
			"fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex,
			"txHash", txHash, "sentTxHash", sentTxHash, "swapNonce", swapTxNonce)
		_ = mongodb.UpdateRouterOldSwapTxs(fromChainID, txid, logIndex, sentTxHash)
	}
	return err
}

func reverifySwap(args *tokens.BuildTxArgs) {
	fromChainID := args.FromChainID.String()
	toChainID := args.ToChainID.String()
	txid := args.SwapID
	logIndex := args.LogIndex
	verifyArgs := &tokens.VerifyArgs{
		SwapType:      args.SwapType,
		LogIndex:      logIndex,
		AllowUnstable: false,
	}
	srcBridge := router.GetBridgeByChainID(fromChainID)
	if srcBridge == nil {
		return
	}
	_, err := srcBridge.VerifyTransaction(txid, verifyArgs)
	switch {
	case err == nil,
		errors.Is(err, tokens.ErrTxNotStable),
		errors.Is(err, tokens.ErrRPCQueryError):
		// ignore the above situations
	default:
		logWorkerWarn("reverify swap after get sign status has disagree", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "err", err)
		_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxNotStable, now(), "")
		_ = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, mongodb.TxNotStable, now(), err.Error())
	}
}

// DeleteCachedSwap delete cached swap
func DeleteCachedSwap(fromChainID, txid string, logIndex int) {
	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
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

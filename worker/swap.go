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
	swapRing        *ring.Ring
	swapRingLock    sync.RWMutex
	swapRingMaxSize = 1000

	cachedSwapTasks    = mapset.NewSet()
	maxCachedSwapTasks = 1000

	swapTaskQueues   = make(map[string]*fifo.Queue) // key is toChainID
	swapTasksInQueue = mapset.NewSet()

	disagreeRecords      = new(sync.Map)
	maxDisagreeCount     = uint64(10)
	disagreeWaitInterval = int64(300)

	errAlreadySwapped     = errors.New("already swapped")
	errSendTxWithDiffHash = errors.New("send tx with different hash")
	errChainIsPaused      = errors.New("from or to chain is paused")
)

// StartSwapJob swap job
func StartSwapJob() {
	// init all swap task queue
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)
		if _, exist := swapTaskQueues[chainID]; !exist {
			swapTaskQueues[chainID] = fifo.NewQueue()
		}
		return true
	})

	// start comsumers
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)

		mongodb.MgoWaitGroup.Add(1)
		go startSwapConsumer(chainID)

		return true
	})

	// start producer
	go startSwapProducer()

}

func startSwapProducer() {
	logWorker("swap", "start router swap job")
	for {
		res, err := findRouterSwapToSwap()
		if err != nil {
			logWorkerError("swap", "find out router swap error", err)
		}
		if len(res) > 0 {
			logWorker("swap", "find out router swap", "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("swap", "stop router swap job")
				return
			}

			if swapTasksInQueue.Contains(swap.Key) {
				logWorkerTrace("swap", "ignore swap in queue", "key", swap.Key)
				continue
			}

			if cachedSwapTasks.Contains(swap.Key) {
				logWorkerTrace("swap", "ignore swap in cache", "key", swap.Key)
				continue
			}

			err = processRouterSwap(swap)
			ctx := []interface{}{"fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex}
			switch {
			case err == nil:
				logWorker("swap", "process router swap success", ctx...)
			case errors.Is(err, errAlreadySwapped),
				errors.Is(err, errChainIsPaused):
				ctx = append(ctx, "err", err)
				logWorkerTrace("swap", "process router swap error", ctx...)
			default:
				logWorkerError("swap", "process router swap error", err, ctx...)
			}
		}
		if utils.IsCleanuping() {
			logWorker("swap", "stop router swap job")
			return
		}
		restInJob(restIntervalInDoSwapJob)
	}
}

func findRouterSwapToSwap() ([]*mongodb.MgoSwap, error) {
	septime := getSepTimeInFind(maxDoSwapLifetime)
	return mongodb.FindRouterSwapsWithStatus(mongodb.TxNotSwapped, septime)
}

func processRouterSwap(swap *mongodb.MgoSwap) (err error) {
	if router.IsChainIDPaused(swap.FromChainID) || router.IsChainIDPaused(swap.ToChainID) {
		return errChainIsPaused
	}

	fromChainID := swap.FromChainID
	toChainID := swap.ToChainID
	txid := swap.TxID
	logIndex := swap.LogIndex
	bind := swap.Bind

	if cachedSwapTasks.Contains(swap.Key) {
		return errAlreadySwapped
	}

	if isBlacked(swap) {
		logWorkerWarn("swap", "swap is in black list", "txid", txid, "logIndex", logIndex, "fromChainID", fromChainID, "toChainID", toChainID, "token", swap.GetToken(), "tokenID", swap.GetTokenID())
		err = tokens.ErrSwapInBlacklist
		_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.SwapInBlacklist, now(), err.Error())
		_ = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, mongodb.SwapInBlacklist, now(), err.Error())
		return nil
	}

	res, err := mongodb.FindRouterSwapResult(fromChainID, txid, logIndex)
	if err != nil {
		if errors.Is(err, mongodb.ErrItemNotFound) {
			_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxNotStable, now(), "")
		}
		return err
	}

	if strings.HasPrefix(res.Memo, tokens.ErrBuildTxErrorAndDelay.Error()) && res.Timestamp+300 > now() {
		return nil
	}

	var disagreeCount uint64
	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
	oldValue, exist := disagreeRecords.Load(cacheKey)
	if exist {
		disagreeCount = oldValue.(uint64)
	}
	if disagreeCount > maxDisagreeCount {
		if res.Timestamp+disagreeWaitInterval > now() {
			logWorkerTrace("swap", "disagree too many times", "txid", txid, "logIndex", logIndex, "fromChainID", fromChainID, "toChainID", toChainID, "token", swap.GetToken(), "tokenID", swap.GetTokenID())
			return nil
		}
		disagreeRecords.Store(cacheKey, 0) // recount from zero
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

	taskQueue, exist := swapTaskQueues[args.ToChainID.String()]
	if !exist {
		return fmt.Errorf("no task queue for chainID '%v'", args.ToChainID)
	}

	logWorker("doSwap", "dispatch router swap task", "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "value", args.OriginValue, "swapNonce", args.GetTxNonce(), "queue", taskQueue.Len())

	taskQueue.Add(args)

	cacheKey := mongodb.GetRouterSwapKey(args.FromChainID.String(), args.SwapID, args.LogIndex)
	swapTasksInQueue.Add(cacheKey)

	return nil
}

func startSwapConsumer(chainID string) {
	defer mongodb.MgoWaitGroup.Done()
	logWorker("doSwap", "start process swap task", "chainID", chainID)

	taskQueue, exist := swapTaskQueues[chainID]
	if !exist {
		log.Fatal("no task queue", "chainID", chainID)
	}

	i := 0
	for {
		if utils.IsCleanuping() {
			logWorker("doSwap", "stop process swap task", "chainID", chainID)
			return
		}

		if i%10 == 0 && taskQueue.Len() > 0 {
			logWorker("doSwap", "tasks in swap queue", "chainID", chainID, "count", taskQueue.Len())
		}
		i++

		front := taskQueue.Next()
		if front == nil {
			sleepSeconds(3)
			continue
		}

		args := front.(*tokens.BuildTxArgs)

		if args.ToChainID.String() != chainID {
			logWorkerWarn("doSwap", "ignore swap task as toChainID mismatch", "want", chainID, "args", args)
			continue
		}
		logWorker("doSwap", "process router swap start", "args", args)
		ctx := []interface{}{"fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex}
		err := doSwap(args)
		switch {
		case err == nil:
			logWorker("doSwap", "process router swap success", ctx...)
		case errors.Is(err, errAlreadySwapped),
			errors.Is(err, tokens.ErrNoBridgeForChainID):
			ctx = append(ctx, "err", err)
			logWorkerTrace("doSwap", "process router swap failed", ctx...)
		default:
			logWorkerError("doSwap", "process router swap failed", err, ctx...)
		}

		cacheKey := mongodb.GetRouterSwapKey(args.FromChainID.String(), args.SwapID, args.LogIndex)
		swapTasksInQueue.Remove(cacheKey)
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
		if errors.Is(err, tokens.ErrBuildTxErrorAndDelay) {
			_ = updateSwapMemo(fromChainID, txid, logIndex, err.Error())
		}
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

	disagreeRecords.Delete(cacheKey)

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
		if errors.Is(err, tokens.ErrBuildTxErrorAndDelay) {
			_ = updateSwapMemo(fromChainID, txid, logIndex, err.Error())
		}
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

	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
	disagreeRecords.Delete(cacheKey)

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
		logWorkerWarn("doSwap", "reverify swap after get sign status has disagree", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "err", err)
		_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxNotStable, now(), "")
		_ = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, mongodb.TxNotStable, now(), err.Error())
	}

	var disagreeCount uint64
	cacheKey := mongodb.GetRouterSwapKey(fromChainID, txid, logIndex)
	oldValue, exist := disagreeRecords.Load(cacheKey)
	if exist {
		disagreeCount = oldValue.(uint64)
	}
	disagreeRecords.Store(cacheKey, disagreeCount+1)
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

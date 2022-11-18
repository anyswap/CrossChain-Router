package worker

import (
	"errors"
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/fifo"
	mapset "github.com/deckarep/golang-set"
)

var (
	verifyTaskQueues   = make(map[string]*fifo.Queue) // key is fromChainID
	verifyTasksInQueue = mapset.NewSet()

	cachedVerifyingSwaps    = mapset.NewSet()
	maxCachedVerifyingSwaps = 100
)

// StartVerifyJob verify job
func StartVerifyJob() {
	logWorker("verify", "start router swap verify job")

	// init all verify task queue
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)

		if _, exist := verifyTaskQueues[chainID]; !exist {
			verifyTaskQueues[chainID] = fifo.NewQueue()
		}

		return true
	})

	// start comsumers
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)

		mongodb.MgoWaitGroup.Add(1)
		go startVerifyConsumer(chainID)

		return true
	})

	// start producer
	go startVerifyProducer()
}

func startVerifyProducer() {
	for {
		septime := getSepTimeInFind(maxVerifyLifetime)
		res, err := mongodb.FindRouterSwapsWithStatus(mongodb.TxNotStable, septime)
		if err != nil {
			logWorkerError("verify", "find router swap error", err)
		}
		if len(res) > 0 {
			logWorker("verify", "find router swap to verify", "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("verify", "stop router swap verify job")
				return
			}

			if verifyTasksInQueue.Contains(swap.Key) {
				logWorkerTrace("verify", "ignore swap in queue", "key", swap.Key)
				continue
			}

			if cachedVerifyingSwaps.Contains(swap.Key) {
				logWorkerTrace("verify", "ignore swap in cache", "key", swap.Key)
				continue
			}

			// minus rpc call to get tx
			if swap, err := mongodb.FindRouterSwap(swap.FromChainID, swap.TxID, swap.LogIndex); err == nil {
				bridge := router.GetBridgeByChainID(swap.FromChainID)
				if bridge != nil && swap.TxHeight > 0 &&
					swap.TxHeight+bridge.GetChainConfig().Confirmations >
						router.GetCachedLatestBlockNumber(swap.FromChainID) {
					logWorkerTrace("verify", "ignore swap not stable", "key", swap.Key)
					continue
				}
			}

			err := dispatchVerifyTask(swap) // produce
			ctx := []interface{}{"fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex}
			if err == nil {
				logWorker("verify", "verify router swap success", ctx...)
			} else {
				logWorkerError("verify", "verify router swap error", err, ctx...)
			}
		}
		restInJob(restIntervalInVerifyJob)
	}
}

func dispatchVerifyTask(swap *mongodb.MgoSwap) error {
	chainID := swap.FromChainID
	taskQueue, exist := verifyTaskQueues[chainID]
	if !exist {
		return fmt.Errorf("no verify task queue for chainID '%v'", chainID)
	}

	logWorker("verify", "dispatch verify swap task", "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "queue", taskQueue.Len())

	taskQueue.Add(swap)
	verifyTasksInQueue.Add(swap.Key)

	return nil
}

func startVerifyConsumer(chainID string) {
	defer mongodb.MgoWaitGroup.Done()
	logWorker("doVerify", "start verify swap task", "chainID", chainID)

	taskQueue, exist := verifyTaskQueues[chainID]
	if !exist {
		log.Fatal("no verify task queue", "chainID", chainID)
	}

	i := 0
	for {
		if utils.IsCleanuping() {
			logWorker("doVerify", "stop verify swap task", "chainID", chainID)
			return
		}

		if i%10 == 0 && taskQueue.Len() > 0 {
			logWorker("doVerify", "tasks in verify queue", "chainID", chainID, "count", taskQueue.Len())
		}
		i++

		front := taskQueue.Next()
		if front == nil {
			sleepSeconds(3)
			continue
		}

		swap := front.(*mongodb.MgoSwap)

		if swap.FromChainID != chainID {
			logWorkerWarn("doVerify", "ignore verify task as fromChainID mismatch", "want", chainID, "swap", swap)
			continue
		}

		go func() {
			ctx := []interface{}{"fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex}
			err := processRouterSwapVerify(swap)
			if err == nil {
				logWorker("doVerify", "verify router swap success", ctx...)
			} else {
				logWorkerError("doVerify", "verify router swap failed", err, ctx...)
			}

			verifyTasksInQueue.Remove(swap.Key)
		}()
	}
}

func isBlacked(swap *mongodb.MgoSwap) bool {
	return params.IsChainIDInBlackList(swap.FromChainID) ||
		params.IsChainIDInBlackList(swap.ToChainID) ||
		params.IsTokenIDInBlackList(swap.GetTokenID()) ||
		params.IsTokenIDInBlackListOnChain(swap.FromChainID, swap.GetTokenID()) ||
		params.IsTokenIDInBlackListOnChain(swap.ToChainID, swap.GetTokenID()) ||
		params.IsAccountInBlackList(swap.From) ||
		params.IsAccountInBlackList(swap.Bind) ||
		params.IsAccountInBlackList(swap.TxTo)
}

//nolint:funlen,gocyclo // ok
func processRouterSwapVerify(swap *mongodb.MgoSwap) (err error) {
	if router.IsChainIDPaused(swap.FromChainID) || router.IsChainIDPaused(swap.ToChainID) {
		return nil
	}

	fromChainID := swap.FromChainID
	txid := swap.TxID
	logIndex := swap.LogIndex

	if cachedVerifyingSwaps.Contains(swap.Key) {
		logWorkerTrace("verify", "ignore swap in cache", "key", swap.Key)
		return nil
	}
	if cachedVerifyingSwaps.Cardinality() >= maxCachedVerifyingSwaps {
		cachedVerifyingSwaps.Pop()
	}
	cachedVerifyingSwaps.Add(swap.Key)
	isProcessed := true
	defer func() {
		if !isProcessed {
			cachedVerifyingSwaps.Remove(swap.Key)
		}
	}()

	var dbErr error
	if isBlacked(swap) {
		err = tokens.ErrSwapInBlacklist
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.SwapInBlacklist, now(), err.Error())
		if dbErr != nil {
			logWorkerError("verify", "verify router swap db error", dbErr, "fromChainID", fromChainID, "toChainID", swap.ToChainID, "txid", txid, "logIndex", logIndex)
		}
		return err
	}

	bridge := router.GetBridgeByChainID(fromChainID)
	if bridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	logWorker("verify", "process swap verify", "fromChainID", fromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)

	verifyArgs := &tokens.VerifyArgs{
		SwapType:      tokens.SwapType(swap.SwapType),
		LogIndex:      logIndex,
		AllowUnstable: false,
	}

	start := time.Now()
	swapInfo, err := bridge.VerifyTransaction(txid, verifyArgs)
	logWorker("verify", "verify tx finished job", "fromChainID", fromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "timespent", time.Since(start).String())

	switch {
	case err == nil:
		if router.IsBigValueSwap(swapInfo) {
			dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithBigValue, now(), "big swap value")
		} else {
			dbErr = addInitialSwapResult(fromChainID, txid, logIndex, swapInfo, mongodb.MatchTxEmpty)
		}
	case errors.Is(err, tokens.ErrTxNotStable),
		errors.Is(err, tokens.ErrRPCQueryError),
		errors.Is(err, tokens.ErrTxNotFound),
		errors.Is(err, tokens.ErrNotFound):
		if swapInfo != nil && swapInfo.Height > 0 {
			_ = mongodb.UpdateRouterSwapHeight(fromChainID, txid, logIndex, swapInfo.Height)
		}
		nowMilli := common.NowMilli()
		if swap.InitTime+1000*maxTxNotFoundTime < nowMilli {
			duration := time.Duration((nowMilli - swap.InitTime) / 1000 * int64(time.Second))
			logWorker("verify", "set longer not found swap to verify failed", "fromChainID", fromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "inittime", swap.InitTime, "duration", duration.String())
			dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxVerifyFailed, now(), err.Error())
			_ = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, mongodb.TxVerifyFailed, now(), err.Error())
		} else {
			isProcessed = false
			return err
		}
	case errors.Is(err, tokens.ErrTxWithWrongValue):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithWrongValue, now(), err.Error())
	case errors.Is(err, tokens.ErrTxWithWrongPath):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithWrongPath, now(), err.Error())
	case errors.Is(err, tokens.ErrMissTokenConfig):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.MissTokenConfig, now(), err.Error())
	case errors.Is(err, tokens.ErrNoUnderlyingToken):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.NoUnderlyingToken, now(), err.Error())
	case errors.Is(err, tokens.ErrVerifyTxUnsafe):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxMaybeUnsafe, now(), err.Error())
	case errors.Is(err, tokens.ErrSwapoutForbidden):
		dbErr = addInitialSwapResult(fromChainID, txid, logIndex, swapInfo, mongodb.SwapoutForbidden)
	default:
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxVerifyFailed, now(), err.Error())
	}

	if dbErr != nil {
		logWorkerError("verify", "verify router swap db error", dbErr, "fromChainID", fromChainID, "toChainID", swap.ToChainID, "txid", txid, "logIndex", logIndex)
	}

	if err != nil {
		logWorkerError("verify", "verify router swap error", err, "fromChainID", fromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
	}

	return err
}

func addInitialSwapResult(
	fromChainID, txid string, logIndex int,
	swapInfo *tokens.SwapTxInfo,
	resStatus mongodb.SwapStatus,
) error {
	err := mongodb.PassRouterSwapVerify(fromChainID, txid, logIndex, now())
	if err != nil {
		return err
	}
	return AddInitialSwapResult(swapInfo, resStatus)
}

// DeleteCachedVerifyingSwap delete cached verifying swap
func DeleteCachedVerifyingSwap(key string) {
	cachedVerifyingSwaps.Remove(key)
}

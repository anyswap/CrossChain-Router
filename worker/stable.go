package worker

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/fifo"
	mapset "github.com/deckarep/golang-set"
)

var (
	stableTaskQueues   = make(map[string]*fifo.Queue) // key is toChainID
	stableTasksInQueue = mapset.NewSet()
)

// StartStableJob stable job
func StartStableJob() {
	logWorker("stable", "start router swap stable job")

	// init all swap stable task queue
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)

		logWorker("swap", "init swap task queue", "chainID", chainID)
		if _, exist := stableTaskQueues[chainID]; !exist {
			stableTaskQueues[chainID] = fifo.NewQueue()
		}
		return true
	})

	// start comsumers
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)

		mongodb.MgoWaitGroup.Add(1)
		go startStableConsumer(chainID)

		return true
	})

	// start producer
	go startStableProducer()
}

func startStableProducer() {
	for {
		res, err := findRouterSwapResultsToStable()
		if err != nil {
			logWorkerError("stable", "find router swap results error", err)
		}
		if len(res) > 0 {
			logWorker("stable", "find router swap results to stable", "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("stable", "stop router swap stable job")
				return
			}

			if stableTasksInQueue.Contains(swap.Key) {
				logWorkerTrace("stable", "ignore swap in queue", "key", swap.Key)
				continue
			}

			err = dispatchSwapResultToStable(swap)
			if err != nil {
				logWorkerError("stable", "process router swap stable error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "toChainID", swap.ToChainID)
			}
		}
		if utils.IsCleanuping() {
			logWorker("stable", "stop router swap stable job")
			return
		}
		restInJob(restIntervalInStableJob)
	}
}

func findRouterSwapResultsToStable() ([]*mongodb.MgoSwapResult, error) {
	septime := getSepTimeInFind(maxStableLifetime)
	return mongodb.FindRouterSwapResultsWithStatus(mongodb.MatchTxNotStable, septime)
}

func isTxOnChain(txStatus *tokens.TxStatus) bool {
	if txStatus == nil || txStatus.BlockHeight == 0 {
		return false
	}
	return txStatus.Receipt != nil
}

func getSwapTxStatus(resBridge tokens.IBridge, swap *mongodb.MgoSwapResult) *tokens.TxStatus {
	txStatus, err := resBridge.GetTransactionStatus(swap.SwapTx)
	if err == nil && isTxOnChain(txStatus) {
		return txStatus
	}
	for _, oldSwapTx := range swap.OldSwapTxs {
		if swap.SwapTx == oldSwapTx {
			continue
		}
		txStatus2, err2 := resBridge.GetTransactionStatus(oldSwapTx)
		if err2 == nil && isTxOnChain(txStatus2) {
			swap.SwapTx = oldSwapTx
			return txStatus2
		}
	}
	return txStatus
}

func dispatchSwapResultToStable(res *mongodb.MgoSwapResult) error {
	chainID := res.ToChainID
	taskQueue, exist := stableTaskQueues[chainID]
	if !exist {
		return fmt.Errorf("no stable task queue for chainID '%v'", chainID)
	}

	logWorker("stable", "dispatch stable router swap task", "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "txid", res.TxID, "logIndex", res.LogIndex, "value", res.SwapValue, "swapNonce", res.SwapNonce, "queue", taskQueue.Len())

	taskQueue.Add(res)
	stableTasksInQueue.Add(res.Key)

	return nil
}

func startStableConsumer(chainID string) {
	defer mongodb.MgoWaitGroup.Done()
	logWorker("doStable", "start process swap task", "chainID", chainID)

	taskQueue, exist := stableTaskQueues[chainID]
	if !exist {
		log.Fatal("no task queue", "chainID", chainID)
	}

	i := 0
	for {
		if utils.IsCleanuping() {
			logWorker("doStable", "stop process swap task", "chainID", chainID)
			return
		}

		if i%10 == 0 {
			logWorker("doStable", "tasks in stable queue", "chainID", chainID, "count", taskQueue.Len())
		}
		i++

		front := taskQueue.Next()
		if front == nil {
			sleepSeconds(3)
			continue
		}

		swap := front.(*mongodb.MgoSwapResult)

		if swap.ToChainID != chainID {
			logWorkerWarn("doStable", "ignore stable task as toChainID mismatch", "want", chainID, "swap", swap)
			continue
		}

		ctx := []interface{}{"fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex}
		err := processRouterSwapStable(swap)
		if err == nil {
			logWorker("doStable", "process router swap success", ctx...)
		} else {
			logWorkerError("doStable", "process router swap failed", err, ctx...)
		}

		stableTasksInQueue.Remove(swap.Key)
	}
}

func processRouterSwapStable(swap *mongodb.MgoSwapResult) (err error) {
	oldSwapTx := swap.SwapTx
	resBridge := router.GetBridgeByChainID(swap.ToChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	txStatus := getSwapTxStatus(resBridge, swap)
	if txStatus == nil || txStatus.BlockHeight == 0 {
		if swap.SwapHeight != 0 {
			return nil
		}
		return checkIfSwapNonceHasPassed(resBridge, swap, false)
	}

	if swap.SwapHeight != 0 {
		if txStatus.Confirmations < resBridge.GetChainConfig().Confirmations {
			return nil
		}
		if swap.SwapTx != oldSwapTx {
			_ = updateSwapTx(swap.FromChainID, swap.TxID, swap.LogIndex, swap.SwapTx)
		}
		if txStatus.IsSwapTxOnChainAndFailed() {
			logWorker("stable", "mark swap result onchain failed",
				"fromChainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex,
				"swaptime", swap.Timestamp, "nowtime", now())
			return markSwapResultFailed(swap.FromChainID, swap.TxID, swap.LogIndex)
		}
		return markSwapResultStable(swap.FromChainID, swap.TxID, swap.LogIndex)
	}

	matchTx := &MatchTx{
		SwapHeight: txStatus.BlockHeight,
		SwapTime:   txStatus.BlockTime,
	}
	if swap.SwapTx != oldSwapTx {
		matchTx.SwapTx = swap.SwapTx
	}
	return updateRouterSwapResult(swap.FromChainID, swap.TxID, swap.LogIndex, matchTx)
}

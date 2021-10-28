package worker

import (
	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// StartStableJob stable job
func StartStableJob() {
	logWorker("stable", "start router swap stable job")

	mongodb.MgoWaitGroup.Add(1)
	go doStableJob()
}

func doStableJob() {
	defer mongodb.MgoWaitGroup.Done()
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
			err = processRouterSwapStable(swap)
			if err != nil {
				logWorkerError("stable", "process router swap stable error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
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
	status := mongodb.MatchTxNotStable
	septime := getSepTimeInFind(maxStableLifetime)
	return mongodb.FindRouterSwapResultsWithStatus(status, septime)
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

func processRouterSwapStable(swap *mongodb.MgoSwapResult) (err error) {
	oldSwapTx := swap.SwapTx
	resBridge := router.GetBridgeByChainID(swap.ToChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	txStatus := getSwapTxStatus(resBridge, swap)
	if txStatus == nil || txStatus.BlockHeight == 0 {
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

package worker

import (
	"github.com/anyswap/CrossChain-Router/mongodb"
	"github.com/anyswap/CrossChain-Router/router"
	"github.com/anyswap/CrossChain-Router/tokens"
	"github.com/anyswap/CrossChain-Router/types"
)

// StartStableJob stable job
func StartStableJob() {
	logWorker("stable", "start router swap stable job")
	for {
		res, err := findRouterSwapResultsToStable()
		if err != nil {
			logWorkerError("stable", "find router swap results error", err)
		}
		if len(res) > 0 {
			logWorker("stable", "find router swap results to stable", "count", len(res))
		}
		for _, swap := range res {
			err = processRouterSwapStable(swap)
			if err != nil {
				logWorkerError("stable", "process router swap stable error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
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
	txStatus := resBridge.GetTransactionStatus(swap.SwapTx)
	if isTxOnChain(txStatus) {
		return txStatus
	}
	for _, oldSwapTx := range swap.OldSwapTxs {
		if swap.SwapTx == oldSwapTx {
			continue
		}
		txStatus2 := resBridge.GetTransactionStatus(oldSwapTx)
		if isTxOnChain(txStatus2) {
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
		if txStatus.Receipt != nil {
			receipt, ok := txStatus.Receipt.(*types.RPCTxReceipt)
			txFailed := !ok || receipt == nil || *receipt.Status != 1 || len(receipt.Logs) == 0
			if txFailed {
				logWorker("[stable]", "mark swap result onchain failed",
					"fromChainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex,
					"swaptime", swap.Timestamp, "nowtime", now())
				return markSwapResultFailed(swap.FromChainID, swap.TxID, swap.LogIndex)
			}
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

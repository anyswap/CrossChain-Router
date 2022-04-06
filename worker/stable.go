package worker

import (
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	stableChanSize          = 50
	routerStableTaskChanMap = make(map[string]chan *mongodb.MgoSwapResult) // key is chainID

	errStableChannelIsFull = errors.New("stable task channel is full")
)

// StartStableJob stable job
func StartStableJob() {
	logWorker("stable", "start router swap stable job")

	for _, bridge := range router.RouterBridges {
		chainID := bridge.GetChainConfig().ChainID
		if _, exist := routerStableTaskChanMap[chainID]; !exist {
			routerStableTaskChanMap[chainID] = make(chan *mongodb.MgoSwapResult, stableChanSize)
			utils.TopWaitGroup.Add(1)
			go processStableTask(chainID, routerStableTaskChanMap[chainID])
		}

		mongodb.MgoWaitGroup.Add(1)
		go startStableJob(chainID)
	}
}

func startStableJob(chainID string) {
	defer mongodb.MgoWaitGroup.Done()
	for {
		res, err := findRouterSwapResultsToStable(chainID)
		if err != nil {
			logWorkerError("stable", "find router swap results error", err, "chainID", chainID)
		}
		if len(res) > 0 {
			logWorker("stable", "find router swap results to stable", "count", len(res), "chainID", chainID)
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("stable", "stop router swap stable job", "chainID", chainID)
				return
			}
			err = dispatchSwapResultToStable(swap)
			if err != nil && !errors.Is(err, errStableChannelIsFull) {
				logWorkerError("stable", "process router swap stable error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "toChainID", chainID)
			}
		}
		if utils.IsCleanuping() {
			logWorker("stable", "stop router swap stable job", "chainID", chainID)
			return
		}
		restInJob(restIntervalInStableJob)
	}
}

func findRouterSwapResultsToStable(chainID string) ([]*mongodb.MgoSwapResult, error) {
	septime := getSepTimeInFind(maxStableLifetime)
	return mongodb.FindRouterSwapResultsToStable(chainID, septime)
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

func dispatchSwapResultToStable(swap *mongodb.MgoSwapResult) error {
	chainID := swap.ToChainID
	ch, exist := routerStableTaskChanMap[chainID]
	if !exist {
		return fmt.Errorf("no stable channel for chainID %v", chainID)
	}

	ctx := []interface{}{
		"fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "value", swap.Value, "swapNonce", swap.SwapNonce,
	}

	select {
	case ch <- swap:
		logWorker("stable", "dispatch router stable task", ctx...)
	default:
		logWorkerWarn("stable", "stable task channel is full", ctx...)
		return errStableChannelIsFull
	}

	return nil
}

func processStableTask(chainID string, swapChan <-chan *mongodb.MgoSwapResult) {
	defer utils.TopWaitGroup.Done()
	for {
		select {
		case <-utils.CleanupChan:
			logWorker("stable", "stop process swap task", "chainID", chainID)
			return
		case swap := <-swapChan:
			if swap.ToChainID != chainID {
				logWorkerWarn("stable", "ignore stable task as toChainID mismatch", "want", chainID, "have", swap.ToChainID)
				continue
			}
			err := processRouterSwapStable(swap)
			switch {
			case err == nil, errors.Is(err, tokens.ErrNoBridgeForChainID):
			default:
				logWorkerError("stable", "process router swap failed", err, "swap", swap)
			}
		}
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

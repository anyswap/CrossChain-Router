package worker

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	minTimeIntervalToPassBigValue = int64(300) // seconds
)

// StartPassBigValueJob pass big value job
func StartPassBigValueJob() {
	logWorker("passbigval", "start pass big value job")
	serverCfg = params.GetRouterServerConfig()
	if serverCfg == nil {
		logWorker("passbigval", "stop pass big value job as no router server config exist")
		return
	}
	if !serverCfg.EnablePassBigValueSwap {
		logWorker("passbigval", "stop pass big value job as disabled")
		return
	}
	if !tokens.IsERC20Router() {
		logWorker("passbigval", "stop pass big value job as non erc20 swap")
		return
	}

	mongodb.MgoWaitGroup.Add(1)
	go doPassBigValueJob()
}

func doPassBigValueJob() {
	defer mongodb.MgoWaitGroup.Done()
	for {
		res, err := findBigValRouterSwaps()
		if err != nil {
			logWorkerError("passbigval", "find big value swaps error", err)
		}
		if len(res) > 0 {
			logWorker("passbigval", "find big value swaps to pass", "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("passbigval", "stop pass big value swaps job")
				return
			}
			err = processPassBigValRouterSwap(swap)
			switch {
			case err == nil,
				errors.Is(err, tokens.ErrTxNotStable),
				errors.Is(err, tokens.ErrTxNotFound):
			default:
				logWorkerError("passbigval", "process pass big value swaps error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		if utils.IsCleanuping() {
			logWorker("passbigval", "stop pass big value swaps job")
			return
		}
		restInJob(restIntervalInPassBigValJob)
	}
}

func findBigValRouterSwaps() ([]*mongodb.MgoSwap, error) {
	status := mongodb.TxWithBigValue
	septime := getSepTimeInFind(maxPassBigValueLifetime)
	return mongodb.FindRouterSwapsWithStatus(status, septime)
}

func processPassBigValRouterSwap(swap *mongodb.MgoSwap) (err error) {
	if swap.Status != mongodb.TxWithBigValue {
		return nil
	}
	if swap.InitTime > getSepTimeInFind(passBigValueTimeRequired)*1000 { // init time is milli seconds
		return nil
	}
	if getSepTimeInFind(minTimeIntervalToPassBigValue) < swap.Timestamp {
		return nil
	}

	fromChainID := swap.FromChainID
	txid := swap.TxID
	logIndex := swap.LogIndex

	_, err = mongodb.FindRouterSwapResult(fromChainID, txid, logIndex)
	if err == nil {
		return nil // result exist
	}

	bridge := router.GetBridgeByChainID(fromChainID)
	if bridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	verifyArgs := &tokens.VerifyArgs{
		SwapType:      tokens.SwapType(swap.SwapType),
		LogIndex:      logIndex,
		AllowUnstable: false,
	}
	swapInfo, err := bridge.VerifyTransaction(txid, verifyArgs)
	if err != nil {
		return err
	}

	err = mongodb.RouterAdminPassBigValue(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}

	_ = AddInitialSwapResult(swapInfo, mongodb.MatchTxEmpty)
	return nil
}

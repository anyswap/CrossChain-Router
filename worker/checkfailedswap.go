package worker

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// StartCheckFailedSwapJob check failed swap job
func StartCheckFailedSwapJob() {
	logWorker("checkfailedswap", "start router check failed swap job")

	mongodb.MgoWaitGroup.Add(1)
	go doCheckFailedSwapJob()
}

func doCheckFailedSwapJob() {
	defer mongodb.MgoWaitGroup.Done()
	for {
		septime := getSepTimeInFind(maxCheckFailedSwapLifetime)
		res, err := mongodb.FindRouterSwapResultsWithStatus(mongodb.MatchTxFailed, septime)
		if err != nil {
			logWorkerError("checkfailedswap", "find failed router swap error", err)
		}
		if len(res) > 0 {
			logWorker("checkfailedswap", "find failed router swap to check", "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("checkfailedswap", "stop check failed router swap job")
				return
			}
			err = checkFailedRouterSwap(swap)
			if err != nil {
				logWorkerError("checkfailedswap", "check failed router swap error", err, "chainid", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		if utils.IsCleanuping() {
			logWorker("checkfailedswap", "stop check failed router swap job")
			return
		}
		restInJob(restIntervalInCheckFailedSwapJob)
	}
}

func checkFailedRouterSwap(swap *mongodb.MgoSwapResult) error {
	if (swap.SwapNonce == 0 && swap.SwapHeight == 0) || swap.SwapTx == "" {
		return nil
	}

	resBridge := router.GetBridgeByChainID(swap.ToChainID)
	if resBridge == nil {
		logWorkerWarn("checkfailedswap", "bridge not exist", "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
		return nil
	}
	nonceSetter, ok := resBridge.(tokens.NonceSetter)
	if !ok {
		return nil
	}
	routerMPC, err := router.GetRouterMPC(swap.GetTokenID(), swap.ToChainID)
	if err != nil {
		return nil
	}
	if !common.IsEqualIgnoreCase(swap.MPC, routerMPC) {
		return tokens.ErrSenderMismatch
	}

	txStatus := getSwapTxStatus(resBridge, swap)
	if txStatus != nil && txStatus.IsSwapTxOnChainAndFailed() {
		return nil
	}

	if txStatus != nil && txStatus.BlockHeight > 0 {
		logWorker("checkfailedswap", "do checking with height", "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "swaptx", swap.SwapTx, "swapnonce", swap.SwapNonce, "swapheight", txStatus.BlockHeight, "confirmations", txStatus.Confirmations)
		if txStatus.Confirmations < resBridge.GetChainConfig().Confirmations {
			return markSwapResultUnstable(swap.FromChainID, swap.TxID, swap.LogIndex)
		}
		return markSwapResultStable(swap.FromChainID, swap.TxID, swap.LogIndex)
	}

	nonce, err := nonceSetter.GetPoolNonce(swap.MPC, "latest")
	if err != nil {
		return fmt.Errorf("get router mpc nonce failed, %w", err)
	}

	if nonce <= swap.SwapNonce {
		logWorker("checkfailedswap", "do checking without height", "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "swaptx", swap.SwapTx, "swapnonce", swap.SwapNonce, "latestnonce", nonce)
		return markSwapResultUnstable(swap.FromChainID, swap.TxID, swap.LogIndex)
	}
	return nil
}

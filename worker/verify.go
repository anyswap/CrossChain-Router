package worker

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// StartVerifyJob verify job
func StartVerifyJob() {
	logWorker("verify", "start router swap verify job")

	mongodb.MgoWaitGroup.Add(1)
	go doVerifyJob()
}

func doVerifyJob() {
	defer mongodb.MgoWaitGroup.Done()
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
			err = processRouterSwapVerify(swap)
			switch {
			case err == nil,
				errors.Is(err, tokens.ErrTxNotStable),
				errors.Is(err, tokens.ErrTxNotFound):
			default:
				logWorkerError("verify", "verify router swap error", err, "chainid", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		if utils.IsCleanuping() {
			logWorker("verify", "stop router swap verify job")
			return
		}
		restInJob(restIntervalInVerifyJob)
	}
}

func processRouterSwapVerify(swap *mongodb.MgoSwap) (err error) {
	fromChainID := swap.FromChainID
	txid := swap.TxID
	logIndex := swap.LogIndex

	var dbErr error
	if params.IsSwapInBlacklist(fromChainID, swap.ToChainID, swap.GetTokenID()) {
		err = tokens.ErrSwapInBlacklist
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.SwapInBlacklist, now(), err.Error())
		if dbErr != nil {
			logWorkerError("verify", "verify router swap db error", dbErr, "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex)
		}
		return err
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
	switch {
	case err == nil:
		if verifyArgs.SwapType == tokens.RouterSwapType &&
			swapInfo.Value.Cmp(tokens.GetBigValueThreshold(swapInfo.TokenID, swap.ToChainID, bridge.GetTokenConfig(swapInfo.Token).Decimals)) > 0 {
			dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithBigValue, now(), "")
		} else {
			dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxNotSwapped, now(), "")
			if dbErr == nil {
				dbErr = AddInitialSwapResult(swapInfo, mongodb.MatchTxEmpty)
			}
		}
	case errors.Is(err, tokens.ErrTxNotStable), errors.Is(err, tokens.ErrTxNotFound):
		break
	case errors.Is(err, tokens.ErrTxWithWrongValue):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithWrongValue, now(), err.Error())
	case errors.Is(err, tokens.ErrTxWithWrongPath):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithWrongPath, now(), err.Error())
	case errors.Is(err, tokens.ErrMissTokenConfig):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.MissTokenConfig, now(), err.Error())
	case errors.Is(err, tokens.ErrNoUnderlyingToken):
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.NoUnderlyingToken, now(), err.Error())
	default:
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxVerifyFailed, now(), err.Error())
	}

	if dbErr != nil {
		logWorkerError("verify", "verify router swap db error", dbErr, "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex)
	}

	return err
}

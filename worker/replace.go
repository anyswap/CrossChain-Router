package worker

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/mongodb"
	"github.com/anyswap/CrossChain-Router/params"
	"github.com/anyswap/CrossChain-Router/router"
	"github.com/anyswap/CrossChain-Router/tokens"
)

var (
	defWaitTimeToReplace = int64(900) // seconds
	defMaxReplaceCount   = 20
)

// StartReplaceJob replace job
func StartReplaceJob() {
	logWorker("replace", "start router swap replace job")
	if !params.GetRouterConfig().Server.EnableReplaceSwap {
		logWorker("replace", "stop replace swap job as disabled")
		return
	}
	for {
		res, err := findRouterSwapResultToReplace()
		if err != nil {
			logWorkerError("replace", "find router swap result error", err)
		}
		for _, swap := range res {
			err = processRouterSwapReplace(swap)
			if err != nil {
				logWorkerError("replace", "process router swap replace error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		restInJob(restIntervalInReplaceSwapJob)
	}
}

func findRouterSwapResultToReplace() ([]*mongodb.MgoSwapResult, error) {
	status := mongodb.MatchTxNotStable
	septime := getSepTimeInFind(maxReplaceSwapLifetime)
	return mongodb.FindRouterSwapResultsWithStatus(status, septime)
}

func processRouterSwapReplace(res *mongodb.MgoSwapResult) error {
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	chainCfg := resBridge.GetChainConfig()
	waitTimeToReplace := chainCfg.WaitTimeToReplace
	maxReplaceCount := chainCfg.MaxReplaceCount
	if waitTimeToReplace == 0 {
		waitTimeToReplace = defWaitTimeToReplace
	}
	if maxReplaceCount == 0 {
		maxReplaceCount = defMaxReplaceCount
	}
	if len(res.OldSwapTxs) > maxReplaceCount {
		return fmt.Errorf("replace swap too many times (> %v)", maxReplaceCount)
	}
	if getSepTimeInFind(waitTimeToReplace) < res.Timestamp {
		return nil
	}
	return ReplaceRouterSwap(res, nil)
}

// ReplaceRouterSwap api
func ReplaceRouterSwap(res *mongodb.MgoSwapResult, gasPrice *big.Int) error {
	swap, err := verifyReplaceSwap(res)
	if err != nil {
		return err
	}

	resBridge := router.GetBridgeByChainID(res.ToChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	biFromChainID, biToChainID, biValue, err := getFromToChainIDAndValue(res.FromChainID, res.ToChainID, res.Value)
	if err != nil {
		return err
	}

	txid := res.TxID
	nonce := res.SwapNonce
	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			Identifier:  params.GetIdentifier(),
			SwapID:      txid,
			SwapType:    tokens.SwapType(res.SwapType),
			Bind:        res.Bind,
			LogIndex:    res.LogIndex,
			FromChainID: biFromChainID,
			ToChainID:   biToChainID,
		},
		From:        resBridge.GetChainConfig().GetRouterMPC(),
		OriginValue: biValue,
		Extra: &tokens.AllExtras{
			EthExtra: &tokens.EthExtraArgs{
				GasPrice: gasPrice,
				Nonce:    &nonce,
			},
		},
	}
	args.SwapInfo, err = mongodb.ConvertFromSwapInfo(&swap.SwapInfo)
	if err != nil {
		return err
	}
	rawTx, err := resBridge.BuildRawTransaction(args)
	if err != nil {
		logWorkerError("replaceSwap", "build tx failed", err, "chainID", res.ToChainID, "txid", txid, "logIndex", res.LogIndex)
		return err
	}
	signedTx, txHash, err := resBridge.MPCSignTransaction(rawTx, args.GetExtraArgs())
	if err != nil {
		return err
	}

	err = replaceSwapResult(res, txHash)
	if err != nil {
		return err
	}
	return sendSignedTransaction(resBridge, signedTx, args, true)
}

func verifyReplaceSwap(res *mongodb.MgoSwapResult) (*mongodb.MgoSwap, error) {
	fromChainID, txid, logIndex := res.FromChainID, res.TxID, res.LogIndex
	swap, err := mongodb.FindRouterSwap(fromChainID, txid, logIndex)
	if err != nil {
		return nil, err
	}
	if res.SwapTx == "" {
		return nil, errors.New("swap without swaptx")
	}
	if res.Status != mongodb.MatchTxNotStable {
		return nil, errors.New("swap result status is not 'MatchTxNotStable'")
	}
	if res.SwapHeight != 0 {
		return nil, errors.New("swaptx with block height")
	}
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	if resBridge == nil {
		return nil, tokens.ErrNoBridgeForChainID
	}
	txStat := getSwapTxStatus(resBridge, res)
	if txStat != nil && txStat.BlockHeight > 0 {
		return nil, errors.New("swaptx exist in chain")
	}

	nonceSetter, ok := resBridge.(tokens.NonceSetter)
	if ok {
		mpc := resBridge.GetChainConfig().GetRouterMPC()
		nonce, err := nonceSetter.GetPoolNonce(mpc, "latest")
		if err != nil {
			return nil, fmt.Errorf("get router mpc nonce failed, %v", err)
		}
		if nonce > res.SwapNonce {
			return nil, fmt.Errorf("can not replace swap with nonce (%v) which is lower than latest nonce (%v)", res.SwapNonce, nonce)
		}
	}

	return swap, nil
}

func replaceSwapResult(swapResult *mongodb.MgoSwapResult, txHash string) (err error) {
	fromChainID := swapResult.FromChainID
	txid := swapResult.TxID
	logIndex := swapResult.LogIndex
	var oldSwapTxs []string
	if len(swapResult.OldSwapTxs) > 0 {
		var existsInOld bool
		for _, oldSwapTx := range swapResult.OldSwapTxs {
			if oldSwapTx == txHash {
				existsInOld = true
				break
			}
		}
		if !existsInOld {
			oldSwapTxs = swapResult.OldSwapTxs
			oldSwapTxs = append(oldSwapTxs, txHash)
		}
	} else if swapResult.SwapTx != "" && txHash != swapResult.SwapTx {
		oldSwapTxs = []string{swapResult.SwapTx, txHash}
	}
	err = updateOldSwapTxs(fromChainID, txid, logIndex, oldSwapTxs)
	if err != nil {
		logWorkerError("replace", "replaceSwapResult failed", err, "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex, "swaptx", txHash, "nonce", swapResult.SwapNonce)
	} else {
		logWorker("replace", "replaceSwapResult success", "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex, "swaptx", txHash, "nonce", swapResult.SwapNonce)
	}
	return err
}

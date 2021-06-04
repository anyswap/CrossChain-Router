package worker

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/cmd/utils"
	"github.com/anyswap/CrossChain-Router/mongodb"
	"github.com/anyswap/CrossChain-Router/params"
	"github.com/anyswap/CrossChain-Router/router"
	"github.com/anyswap/CrossChain-Router/tokens"
)

var (
	serverCfg *params.RouterServerConfig

	treatAsNoncePassedInterval = int64(300) // seconds
	defWaitTimeToReplace       = int64(300) // seconds
	defMaxReplaceCount         = 20

	updateOldSwapTxsLock sync.Mutex
)

// StartReplaceJob replace job
func StartReplaceJob() {
	logWorker("replace", "start router swap replace job")
	serverCfg = params.GetRouterServerConfig()
	if serverCfg == nil {
		logWorker("replace", "stop replace swap job as no router server config exist")
		return
	}
	if !serverCfg.EnableReplaceSwap {
		logWorker("replace", "stop replace swap job as disabled")
		return
	}
	for {
		res, err := findRouterSwapResultToReplace()
		if err != nil {
			logWorkerError("replace", "find router swap result error", err)
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("replace", "stop router swap replace job")
				return
			}
			err = processRouterSwapReplace(swap)
			if err != nil {
				logWorkerError("replace", "process router swap replace error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		restInJob(restIntervalInReplaceSwapJob)
	}
}

func findRouterSwapResultToReplace() ([]*mongodb.MgoSwapResult, error) {
	septime := getSepTimeInFind(maxReplaceSwapLifetime)
	return mongodb.FindRouterSwapResultsToReplace(septime)
}

func processRouterSwapReplace(res *mongodb.MgoSwapResult) error {
	waitTimeToReplace := serverCfg.WaitTimeToReplace
	maxReplaceCount := serverCfg.MaxReplaceCount
	if waitTimeToReplace == 0 {
		waitTimeToReplace = defWaitTimeToReplace
	}
	if maxReplaceCount == 0 {
		maxReplaceCount = defMaxReplaceCount
	}
	if len(res.OldSwapTxs) > maxReplaceCount {
		return nil
	}
	if getSepTimeInFind(waitTimeToReplace) < res.Timestamp {
		return nil
	}
	resBridge := router.GetBridgeByChainID(res.ToChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	err := checkIfSwapNonceHasPassed(resBridge, res, true)
	if err != nil {
		return err
	}
	_ = updateSwapTimestamp(res.FromChainID, res.TxID, res.LogIndex)
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

	logWorker("replaceSwap", "process task", "swap", res)

	txid := res.TxID
	nonce := res.SwapNonce
	replaceNum := uint64(len(res.OldSwapTxs))
	if replaceNum == 0 {
		replaceNum++
	}
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
		ReplaceNum:  replaceNum,
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
	sentTxHash, err := sendSignedTransaction(resBridge, signedTx, args)
	if err == nil && txHash != sentTxHash {
		logWorkerError("replaceSwap", "send tx success but with different hash", errSendTxWithDiffHash, "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "txid", txid, "logIndex", res.LogIndex, "txHash", txHash, "sentTxHash", sentTxHash)
		_ = replaceSwapResult(res, sentTxHash)
	}
	return err
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
	err = checkIfSwapNonceHasPassed(resBridge, res, true)
	if err != nil {
		return nil, err
	}
	return swap, nil
}

func checkIfSwapNonceHasPassed(bridge tokens.IBridge, res *mongodb.MgoSwapResult, isReplace bool) error {
	nonceSetter, ok := bridge.(tokens.NonceSetter)
	if !ok {
		return nil
	}
	mpc := bridge.GetChainConfig().GetRouterMPC()
	nonce, err := nonceSetter.GetPoolNonce(mpc, "latest")
	if err != nil {
		return fmt.Errorf("get router mpc nonce failed, %w", err)
	}
	if nonce > res.SwapNonce && res.SwapNonce > 0 {
		var iden string
		if isReplace {
			iden = "[replace]"
		} else {
			iden = "[stable]"
		}
		fromChainID, txid, logIndex := res.FromChainID, res.TxID, res.LogIndex
		if res.Timestamp < getSepTimeInFind(treatAsNoncePassedInterval) {
			logWorker(iden, "mark swap result nonce passed",
				"fromChainID", fromChainID, "txid", txid, "logIndex", logIndex,
				"swaptime", res.Timestamp, "nowtime", now())
			_ = markSwapResultFailed(fromChainID, txid, logIndex)
		}
		if isReplace {
			return fmt.Errorf("swap nonce (%v) is lower than latest nonce (%v)", res.SwapNonce, nonce)
		}
	}
	return nil
}

func replaceSwapResult(swapResult *mongodb.MgoSwapResult, txHash string) (err error) {
	updateOldSwapTxsLock.Lock()
	defer updateOldSwapTxsLock.Unlock()

	fromChainID := swapResult.FromChainID
	txid := swapResult.TxID
	logIndex := swapResult.LogIndex
	var oldSwapTxs []string
	if len(swapResult.OldSwapTxs) > 0 {
		for _, oldSwapTx := range swapResult.OldSwapTxs {
			if oldSwapTx == txHash {
				return nil
			}
		}
		oldSwapTxs = swapResult.OldSwapTxs
		oldSwapTxs = append(oldSwapTxs, txHash)
	} else {
		if txHash == swapResult.SwapTx {
			return nil
		}
		if swapResult.SwapTx == "" {
			oldSwapTxs = []string{txHash}
		} else {
			oldSwapTxs = []string{swapResult.SwapTx, txHash}
		}
	}
	err = updateOldSwapTxs(fromChainID, txid, logIndex, oldSwapTxs)
	if err != nil {
		logWorkerError("replace", "replaceSwapResult failed", err, "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex, "swaptx", txHash, "nonce", swapResult.SwapNonce)
	} else {
		logWorker("replace", "replaceSwapResult success", "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex, "swaptx", txHash, "nonce", swapResult.SwapNonce)
	}
	return err
}

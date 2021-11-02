package worker

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	serverCfg *params.RouterServerConfig

	treatAsNoncePassedInterval = int64(600) // seconds
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

	allChainIDs := router.AllChainIDs
	mongodb.MgoWaitGroup.Add(len(allChainIDs))
	for _, toChainID := range allChainIDs {
		go doReplaceJob(toChainID)
	}
}

func doReplaceJob(toChainID *big.Int) {
	defer mongodb.MgoWaitGroup.Done()
	logWorker("replace", "start router swap replace job", "toChainID", toChainID)
	for {
		res, err := findRouterSwapResultToReplace(toChainID)
		if err != nil {
			logWorkerError("replace", "find router swap result error", err, "toChainID", toChainID)
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				logWorker("replace", "stop router swap replace job", "toChainID", toChainID)
				return
			}
			err = processRouterSwapReplace(swap)
			if err != nil {
				logWorkerError("replace", "process router swap replace error", err, "fromChainID", swap.FromChainID, "toChainID", toChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		if utils.IsCleanuping() {
			logWorker("replace", "stop router swap replace job", "toChainID", toChainID)
			return
		}
		restInJob(restIntervalInReplaceSwapJob)
	}
}

func findRouterSwapResultToReplace(toChainID *big.Int) ([]*mongodb.MgoSwapResult, error) {
	septime := getSepTimeInFind(maxReplaceSwapLifetime)
	resBridge := router.GetBridgeByChainID(toChainID.String())
	mpcAddress := resBridge.GetChainConfig().GetRouterMPC()
	return mongodb.FindRouterSwapResultsToReplace(toChainID, septime, mpcAddress)
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
	return ReplaceRouterSwap(res, nil, false)
}

// ReplaceRouterSwap api
func ReplaceRouterSwap(res *mongodb.MgoSwapResult, gasPrice *big.Int, isManual bool) error {
	swap, err := verifyReplaceSwap(res, isManual)
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
	_ = updateSwapTimestamp(res.FromChainID, res.TxID, res.LogIndex)

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
		OriginFrom:  swap.From,
		OriginTxTo:  swap.TxTo,
		OriginValue: biValue,
		Extra: &tokens.AllExtras{
			EthExtra: &tokens.EthExtraArgs{
				GasPrice: gasPrice,
				Nonce:    &nonce,
			},
			ReplaceNum: replaceNum,
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
	go signAndSendReplaceTx(resBridge, rawTx, args, res)
	return nil
}

func signAndSendReplaceTx(resBridge tokens.IBridge, rawTx interface{}, args *tokens.BuildTxArgs, res *mongodb.MgoSwapResult) {
	signedTx, txHash, err := resBridge.MPCSignTransaction(rawTx, args.GetExtraArgs())
	if err != nil {
		logWorkerError("replaceSwap", "mpc sign tx failed", err, "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "txid", res.TxID, "nonce", res.SwapNonce, "logIndex", res.LogIndex)
		return
	}

	err = replaceSwapResult(res, txHash)
	if err != nil {
		return
	}

	sentTxHash, err := sendSignedTransaction(resBridge, signedTx, args)
	if err == nil && txHash != sentTxHash {
		logWorkerError("replaceSwap", "send tx success but with different hash", errSendTxWithDiffHash, "fromChainID", res.FromChainID, "toChainID", res.ToChainID, "txid", res.TxID, "nonce", res.SwapNonce, "logIndex", res.LogIndex, "txHash", txHash, "sentTxHash", sentTxHash)
		_ = replaceSwapResult(res, sentTxHash)
	}
}

func verifyReplaceSwap(res *mongodb.MgoSwapResult, isManual bool) (*mongodb.MgoSwap, error) {
	fromChainID, txid, logIndex := res.FromChainID, res.TxID, res.LogIndex
	swap, err := mongodb.FindRouterSwap(fromChainID, txid, logIndex)
	if err != nil {
		return nil, err
	}
	if res.SwapTx == "" {
		return nil, errors.New("swap without swaptx")
	}
	if res.SwapNonce == 0 && !isManual {
		return nil, errors.New("swap nonce is zero")
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
	err = checkIfSwapNonceHasPassed(resBridge, res, true)
	if err != nil {
		return nil, err
	}
	return swap, nil
}

func checkIfSwapNonceHasPassed(bridge tokens.IBridge, res *mongodb.MgoSwapResult, isReplace bool) error {
	if res.SwapHeight != 0 {
		if isReplace {
			return errors.New("swaptx with block height")
		}
		return nil
	}
	nonceSetter, ok := bridge.(tokens.NonceSetter)
	if !ok {
		return nil
	}
	mpc := bridge.GetChainConfig().GetRouterMPC()
	nonce, err := nonceSetter.GetPoolNonce(mpc, "latest")
	if err != nil {
		return fmt.Errorf("get router mpc nonce failed, %w", err)
	}
	txStat := getSwapTxStatus(bridge, res)
	if txStat != nil && txStat.BlockHeight > 0 {
		if isReplace {
			return errors.New("swaptx exist in chain")
		}
		return nil
	}
	if nonce > res.SwapNonce && res.SwapNonce > 0 {
		var iden string
		if isReplace {
			iden = "[replace]"
		} else {
			iden = "[stable]"
		}
		fromChainID, txid, logIndex := res.FromChainID, res.TxID, res.LogIndex
		noncePassedInterval := params.GetNoncePassedConfirmInterval(res.FromChainID)
		if noncePassedInterval == 0 {
			noncePassedInterval = treatAsNoncePassedInterval
		}
		if res.Timestamp < getSepTimeInFind(noncePassedInterval) {
			if txStat == nil { // retry to get swap status
				txStat = getSwapTxStatus(bridge, res)
				if txStat != nil && txStat.BlockHeight > 0 {
					if isReplace {
						return errors.New("swaptx exist in chain")
					}
					return nil
				}
			}
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

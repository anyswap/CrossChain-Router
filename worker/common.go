package worker

import (
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// MatchTx struct
type MatchTx struct {
	MPC        string
	SwapTx     string
	SwapHeight uint64
	SwapTime   uint64
	SwapValue  string
	SwapNonce  uint64
}

// AddInitialSwapResult add initial result
func AddInitialSwapResult(swapInfo *tokens.SwapTxInfo, status mongodb.SwapStatus) (err error) {
	valueStr := "0"
	if swapInfo.Value != nil {
		valueStr = swapInfo.Value.String()
	}
	swapResult := &mongodb.MgoSwapResult{
		SwapType:    uint32(swapInfo.SwapType),
		TxID:        swapInfo.Hash,
		TxTo:        swapInfo.TxTo,
		TxHeight:    swapInfo.Height,
		TxTime:      swapInfo.Timestamp,
		From:        swapInfo.From,
		To:          swapInfo.To,
		Bind:        swapInfo.Bind,
		Value:       valueStr,
		LogIndex:    swapInfo.LogIndex,
		FromChainID: swapInfo.FromChainID.String(),
		ToChainID:   swapInfo.ToChainID.String(),
		SwapTx:      "",
		SwapHeight:  0,
		SwapTime:    0,
		SwapValue:   "0",
		SwapNonce:   0,
		Status:      status,
		Timestamp:   now(),
		Memo:        "",
	}
	swapResult.SwapInfo = mongodb.ConvertToSwapInfo(&swapInfo.SwapInfo)
	err = mongodb.AddRouterSwapResult(swapResult)
	if err != nil {
		logWorkerError("add", "addInitialSwapResult failed", err, "chainid", swapInfo.FromChainID, "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex)
	} else {
		logWorker("add", "addInitialSwapResult success", "chainid", swapInfo.FromChainID, "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex)
	}
	return err
}

func updateRouterSwapResult(fromChainID, txid string, logIndex int, mtx *MatchTx) (err error) {
	updates := &mongodb.SwapResultUpdateItems{
		Status:    mongodb.KeepStatus,
		Timestamp: now(),
	}
	if mtx.SwapHeight == 0 {
		updates.SwapValue = mtx.SwapValue
		updates.SwapNonce = mtx.SwapNonce
		updates.SwapHeight = 0
		updates.SwapTime = 0
		if mtx.SwapTx != "" {
			updates.MPC = mtx.MPC
			updates.SwapTx = mtx.SwapTx
			updates.Status = mongodb.MatchTxNotStable
		}
	} else {
		updates.SwapNonce = mtx.SwapNonce
		updates.SwapHeight = mtx.SwapHeight
		updates.SwapTime = mtx.SwapTime
		if mtx.SwapTx != "" {
			updates.SwapTx = mtx.SwapTx
		}
	}
	err = mongodb.UpdateRouterSwapResult(fromChainID, txid, logIndex, updates)
	if err != nil {
		logWorkerError("update", "updateSwapResult failed", err,
			"chainid", fromChainID, "txid", txid, "logIndex", logIndex,
			"swaptx", mtx.SwapTx, "swapheight", mtx.SwapHeight,
			"swaptime", mtx.SwapTime, "swapvalue", mtx.SwapValue,
			"swapnonce", mtx.SwapNonce)
	} else {
		logWorker("update", "updateSwapResult success",
			"chainid", fromChainID, "txid", txid, "logIndex", logIndex,
			"swaptx", mtx.SwapTx, "swapheight", mtx.SwapHeight,
			"swaptime", mtx.SwapTime, "swapvalue", mtx.SwapValue,
			"swapnonce", mtx.SwapNonce)
	}
	return err
}

func updateSwapTimestamp(fromChainID, txid string, logIndex int) (err error) {
	updates := &mongodb.SwapResultUpdateItems{
		Status:    mongodb.KeepStatus,
		Timestamp: now(),
	}
	err = mongodb.UpdateRouterSwapResult(fromChainID, txid, logIndex, updates)
	if err != nil {
		logWorkerError("update", "updateSwapTimestamp failed", err, "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	} else {
		logWorker("update", "updateSwapTimestamp success", "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	}
	return err
}

func updateSwapTx(fromChainID, txid string, logIndex int, swapTx string) (err error) {
	updates := &mongodb.SwapResultUpdateItems{
		Status:    mongodb.KeepStatus,
		SwapTx:    swapTx,
		Timestamp: now(),
	}
	err = mongodb.UpdateRouterSwapResult(fromChainID, txid, logIndex, updates)
	if err != nil {
		logWorkerError("update", "updateSwapTx failed", err, "chainid", fromChainID, "txid", txid, "logIndex", logIndex, "swaptx", swapTx)
	} else {
		logWorker("update", "updateSwapTx success", "chainid", fromChainID, "txid", txid, "logIndex", logIndex, "swaptx", swapTx)
	}
	return err
}

func markSwapResultUnstable(fromChainID, txid string, logIndex int) (err error) {
	status := mongodb.MatchTxNotStable
	timestamp := now()
	memo := "" // unchange
	err = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, status, timestamp, memo)
	if err != nil {
		logWorkerError("checkfailedswap", "markSwapResultUnstable failed", err, "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	} else {
		logWorker("checkfailedswap", "markSwapResultUnstable success", "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	}
	return err
}

func markSwapResultStable(fromChainID, txid string, logIndex int) (err error) {
	status := mongodb.MatchTxStable
	timestamp := now()
	memo := "" // unchange
	err = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, status, timestamp, memo)
	if err != nil {
		logWorkerError("stable", "markSwapResultStable failed", err, "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	} else {
		logWorker("stable", "markSwapResultStable success", "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	}
	return err
}

func markSwapResultFailed(fromChainID, txid string, logIndex int) (err error) {
	status := mongodb.MatchTxFailed
	timestamp := now()
	memo := "" // unchange
	err = mongodb.UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, status, timestamp, memo)
	if err != nil {
		logWorkerError("stable", "markSwapResultFailed failed", err, "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	} else {
		logWorker("stable", "markSwapResultFailed success", "chainid", fromChainID, "txid", txid, "logIndex", logIndex)
	}
	return err
}

func sendSignedTransaction(bridge tokens.IBridge, signedTx interface{}, args *tokens.BuildTxArgs) (txHash string, err error) {
	var (
		retrySendTxCount = 3
		swapTxNonce      = args.GetTxNonce()
		replaceNum       = args.GetReplaceNum()
	)

SENDTX_LOOP:
	for {
		for i := 0; i < retrySendTxCount; i++ {
			txHash, err = bridge.SendTransaction(signedTx)
			if err == nil {
				logWorker("sendtx", "send tx success", "txHash", txHash, "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "swapNonce", swapTxNonce, "replaceNum", replaceNum)
				break SENDTX_LOOP
			}
			sleepSeconds(1)
		}

		// prevent sendtx failed cause many same swap nonce allocation
		if err == nil || !needRetrySendTx(err) {
			break
		}
		logWorkerWarn("sendtx", "send tx failed and will retry", "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "swapNonce", swapTxNonce, "replaceNum", replaceNum, "err", err)
		sleepSeconds(10)
	}

	if err != nil {
		logWorkerError("sendtx", "send tx failed", err, "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "swapNonce", swapTxNonce, "replaceNum", replaceNum)
		return txHash, err
	}

	// update swap result tx height in goroutine
	go func() {
		var txStatus *tokens.TxStatus
		var errt error
		for i := int64(0); i < 10; i++ {
			txStatus, errt = bridge.GetTransactionStatus(txHash)
			if errt == nil && txStatus.BlockHeight > 0 {
				break
			}
			sleepSeconds(5)
		}
		if errt == nil && txStatus.BlockHeight > 0 {
			matchTx := &MatchTx{
				SwapTx:     txHash,
				SwapHeight: txStatus.BlockHeight,
				SwapTime:   txStatus.BlockTime,
			}
			_ = updateRouterSwapResult(args.FromChainID.String(), args.SwapID, args.LogIndex, matchTx)
		}
	}()

	return txHash, err
}

func needRetrySendTx(err error) bool {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "Client.Timeout exceeded while awaiting headers"): // timeout
	case strings.Contains(errMsg, "json-rpc error -32000, internal"): // cronos specific
	default:
		return false
	}
	return true
}

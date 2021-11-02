package worker

import (
	"time"

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
	swapResult := &mongodb.MgoSwapResult{
		SwapType:    uint32(swapInfo.SwapType),
		TxID:        swapInfo.Hash,
		TxTo:        swapInfo.TxTo,
		TxHeight:    swapInfo.Height,
		TxTime:      swapInfo.Timestamp,
		From:        swapInfo.From,
		To:          swapInfo.To,
		Bind:        swapInfo.Bind,
		Value:       swapInfo.Value.String(),
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

func updateOldSwapTxs(fromChainID, txid string, logIndex int, oldSwapTxs []string) (err error) {
	updates := &mongodb.SwapResultUpdateItems{
		Status:     mongodb.KeepStatus,
		OldSwapTxs: oldSwapTxs,
		Timestamp:  now(),
	}
	err = mongodb.UpdateRouterSwapResult(fromChainID, txid, logIndex, updates)
	if err != nil {
		logWorkerError("update", "updateOldSwapTxs fialed", err, "chainid", fromChainID, "txid", txid, "logIndex", logIndex, "swaptxs", len(oldSwapTxs))
	} else {
		logWorker("update", "updateOldSwapTxs success", "chainid", fromChainID, "txid", txid, "logIndex", logIndex, "swaptxs", len(oldSwapTxs))
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
		retrySendTxCount    = 3
		retrySendTxInterval = 1 * time.Second
		swapTxNonce         = args.GetTxNonce()
		replaceNum          = args.GetReplaceNum()
	)
	for i := 0; i < retrySendTxCount; i++ {
		txHash, err = bridge.SendTransaction(signedTx)
		if err == nil {
			logWorker("sendtx", "send tx success", "txHash", txHash, "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "swapNonce", swapTxNonce, "replaceNum", replaceNum)
			break
		}
		time.Sleep(retrySendTxInterval)
	}
	if err != nil {
		logWorkerError("sendtx", "send tx failed", err, "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "txid", args.SwapID, "logIndex", args.LogIndex, "swapNonce", swapTxNonce, "replaceNum", replaceNum)
		return txHash, err
	}

	if nonceSetter, ok := bridge.(tokens.NonceSetter); ok {
		nonceSetter.SetNonce(args.From, swapTxNonce+1)
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
			time.Sleep(5 * time.Second)
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

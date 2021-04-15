package swapapi

import (
	"github.com/anyswap/CrossChain-Router/mongodb"
	"github.com/anyswap/CrossChain-Router/router"
)

// ConvertMgoSwapToSwapInfo convert
func ConvertMgoSwapToSwapInfo(ms *mongodb.MgoSwap) *SwapInfo {
	return &SwapInfo{
		SwapType:    ms.SwapType,
		TxID:        ms.TxID,
		TxTo:        ms.TxTo,
		Bind:        ms.Bind,
		LogIndex:    ms.LogIndex,
		FromChainID: ms.FromChainID,
		ToChainID:   ms.ToChainID,
		SwapInfo:    ms.SwapInfo,
		Status:      ms.Status,
		StatusMsg:   ms.Status.String(),
		Timestamp:   ms.Timestamp,
		Memo:        ms.Memo,
	}
}

// ConvertMgoSwapsToSwapInfos convert
func ConvertMgoSwapsToSwapInfos(msSlice []*mongodb.MgoSwap) []*SwapInfo {
	result := make([]*SwapInfo, len(msSlice))
	for k, v := range msSlice {
		result[k] = ConvertMgoSwapToSwapInfo(v)
	}
	return result
}

// ConvertMgoSwapResultToSwapInfo convert
func ConvertMgoSwapResultToSwapInfo(mr *mongodb.MgoSwapResult) *SwapInfo {
	var confirmations uint64
	if mr.SwapHeight != 0 {
		resBridge := router.GetBridgeByChainID(mr.ToChainID)
		if resBridge != nil {
			latest, _ := resBridge.GetLatestBlockNumber()
			if latest > mr.SwapHeight {
				confirmations = latest - mr.SwapHeight
			}
		}
	}
	return &SwapInfo{
		SwapType:      mr.SwapType,
		TxID:          mr.TxID,
		TxTo:          mr.TxTo,
		TxHeight:      mr.TxHeight,
		TxTime:        mr.TxTime,
		From:          mr.From,
		To:            mr.To,
		Bind:          mr.Bind,
		Value:         mr.Value,
		LogIndex:      mr.LogIndex,
		FromChainID:   mr.FromChainID,
		ToChainID:     mr.ToChainID,
		SwapInfo:      mr.SwapInfo,
		SwapTx:        mr.SwapTx,
		OldSwapTxs:    mr.OldSwapTxs,
		SwapHeight:    mr.SwapHeight,
		SwapTime:      mr.SwapTime,
		SwapValue:     mr.SwapValue,
		SwapNonce:     mr.SwapNonce,
		Status:        mr.Status,
		StatusMsg:     mr.Status.String(),
		Timestamp:     mr.Timestamp,
		Memo:          mr.Memo,
		Confirmations: confirmations,
	}
}

// ConvertMgoSwapResultsToSwapInfos convert
func ConvertMgoSwapResultsToSwapInfos(mrSlice []*mongodb.MgoSwapResult) []*SwapInfo {
	result := make([]*SwapInfo, len(mrSlice))
	for k, v := range mrSlice {
		result[k] = ConvertMgoSwapResultToSwapInfo(v)
	}
	return result
}

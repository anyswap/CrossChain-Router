package swapapi

import (
	"github.com/anyswap/CrossChain-Router/mongodb"
	"github.com/anyswap/CrossChain-Router/router"
)

// ConvertMgoSwapToSwapInfo convert
func ConvertMgoSwapToSwapInfo(ms *mongodb.MgoSwap) *SwapInfo {
	return &SwapInfo{
		TxID:          ms.TxID,
		TxTo:          ms.TxTo,
		Bind:          ms.Bind,
		ForNative:     ms.ForNative,
		ForUnderlying: ms.ForUnderlying,
		Token:         ms.Token,
		TokenID:       ms.TokenID,
		Path:          ms.Path,
		AmountOutMin:  ms.AmountOutMin,
		FromChainID:   ms.FromChainID,
		ToChainID:     ms.ToChainID,
		LogIndex:      ms.LogIndex,
		Status:        ms.Status,
		StatusMsg:     ms.Status.String(),
		Timestamp:     ms.Timestamp,
		Memo:          ms.Memo,
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
		latest, _ := router.GetBridgeByChainID(mr.ToChainID).GetLatestBlockNumber()
		if latest > mr.SwapHeight {
			confirmations = latest - mr.SwapHeight
		}
	}
	return &SwapInfo{
		TxID:          mr.TxID,
		TxTo:          mr.TxTo,
		TxHeight:      mr.TxHeight,
		TxTime:        mr.TxTime,
		From:          mr.From,
		To:            mr.To,
		Bind:          mr.Bind,
		Value:         mr.Value,
		ForNative:     mr.ForNative,
		ForUnderlying: mr.ForUnderlying,
		Token:         mr.Token,
		TokenID:       mr.TokenID,
		Path:          mr.Path,
		AmountOutMin:  mr.AmountOutMin,
		FromChainID:   mr.FromChainID,
		ToChainID:     mr.ToChainID,
		LogIndex:      mr.LogIndex,
		SwapTx:        mr.SwapTx,
		SwapHeight:    mr.SwapHeight,
		SwapTime:      mr.SwapTime,
		SwapValue:     mr.SwapValue,
		SwapType:      mr.SwapType,
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

package mongodb

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func convertToSwapResults(swaps []*MgoSwap) []*MgoSwapResult {
	result := make([]*MgoSwapResult, len(swaps))
	for i, swap := range swaps {
		result[i] = swap.ToSwapResult()
	}
	return result
}

// ConvertToSwapInfo convert
func ConvertToSwapInfo(info *tokens.SwapInfo) SwapInfo {
	swapinfo := SwapInfo{}
	switch {
	case info.ERC20SwapInfo != nil:
		erc20SwapInfo := info.ERC20SwapInfo
		swapinfo.ERC20SwapInfo = &ERC20SwapInfo{
			ForNative:     erc20SwapInfo.ForNative,
			ForUnderlying: erc20SwapInfo.ForUnderlying,
			Token:         erc20SwapInfo.Token,
			TokenID:       erc20SwapInfo.TokenID,
			Path:          erc20SwapInfo.Path,
		}
		if erc20SwapInfo.AmountOutMin != nil {
			swapinfo.ERC20SwapInfo.AmountOutMin = erc20SwapInfo.AmountOutMin.String()
		}
	case info.NFTSwapInfo != nil:
		nftSwapInfo := info.NFTSwapInfo
		swapinfo.NFTSwapInfo = &NFTSwapInfo{
			Token:   nftSwapInfo.Token,
			TokenID: nftSwapInfo.TokenID,
			IDs:     fromBigIntSlice(nftSwapInfo.IDs),
			Amounts: fromBigIntSlice(nftSwapInfo.Amounts),
			Batch:   nftSwapInfo.Batch,
		}
	case info.AnyCallSwapInfo != nil:
		anycallSwapInfo := info.AnyCallSwapInfo
		swapinfo.AnyCallSwapInfo = &AnyCallSwapInfo{
			CallFrom:   anycallSwapInfo.CallFrom,
			CallTo:     anycallSwapInfo.CallTo,
			CallData:   fromHexBytesSlice(anycallSwapInfo.CallData),
			Callbacks:  anycallSwapInfo.Callbacks,
			CallNonces: fromBigIntSlice(anycallSwapInfo.CallNonces),
		}
	}
	return swapinfo
}

// ConvertFromSwapInfo convert
func ConvertFromSwapInfo(swapinfo *SwapInfo) (tokens.SwapInfo, error) {
	info := tokens.SwapInfo{}
	switch {
	case swapinfo.ERC20SwapInfo != nil:
		erc20SwapInfo := swapinfo.ERC20SwapInfo
		var amountOutMin *big.Int
		var err error
		if len(erc20SwapInfo.Path) > 0 {
			amountOutMin, err = common.GetBigIntFromStr(erc20SwapInfo.AmountOutMin)
			if err != nil {
				return info, fmt.Errorf("wrong amountOutMin %v", erc20SwapInfo.AmountOutMin)
			}
		}
		info.ERC20SwapInfo = &tokens.ERC20SwapInfo{
			ForNative:     erc20SwapInfo.ForNative,
			ForUnderlying: erc20SwapInfo.ForUnderlying,
			Token:         erc20SwapInfo.Token,
			TokenID:       erc20SwapInfo.TokenID,
			Path:          erc20SwapInfo.Path,
			AmountOutMin:  amountOutMin,
		}
	case swapinfo.NFTSwapInfo != nil:
		nftSwapInfo := swapinfo.NFTSwapInfo
		ids, err := toBigIntSlice(nftSwapInfo.IDs)
		if err != nil {
			return info, fmt.Errorf("wrong ids %v", nftSwapInfo.IDs)
		}
		amounts, err := toBigIntSlice(nftSwapInfo.Amounts)
		if err != nil {
			return info, fmt.Errorf("wrong amounts %v", nftSwapInfo.Amounts)
		}
		info.NFTSwapInfo = &tokens.NFTSwapInfo{
			Token:   nftSwapInfo.Token,
			TokenID: nftSwapInfo.TokenID,
			IDs:     ids,
			Amounts: amounts,
			Batch:   nftSwapInfo.Batch,
		}
	case swapinfo.AnyCallSwapInfo != nil:
		anyCallSwapInfo := swapinfo.AnyCallSwapInfo
		nonces, err := toBigIntSlice(anyCallSwapInfo.CallNonces)
		if err != nil {
			return info, fmt.Errorf("wrong nonces %v", anyCallSwapInfo.CallNonces)
		}
		info.AnyCallSwapInfo = &tokens.AnyCallSwapInfo{
			CallFrom:   anyCallSwapInfo.CallFrom,
			CallTo:     anyCallSwapInfo.CallTo,
			CallData:   toHexBytesSlice(anyCallSwapInfo.CallData),
			Callbacks:  anyCallSwapInfo.Callbacks,
			CallNonces: nonces,
		}
	}
	return info, nil
}

func fromHexBytesSlice(slice []hexutil.Bytes) []string {
	result := make([]string, len(slice))
	for i, elem := range slice {
		result[i] = elem.String()
	}
	return result
}

func fromBigIntSlice(slice []*big.Int) []string {
	result := make([]string, len(slice))
	for i, elem := range slice {
		result[i] = elem.String()
	}
	return result
}

func toHexBytesSlice(slice []string) []hexutil.Bytes {
	result := make([]hexutil.Bytes, len(slice))
	for i, s := range slice {
		result[i] = common.FromHex(s)
	}
	return result
}

func toBigIntSlice(slice []string) ([]*big.Int, error) {
	result := make([]*big.Int, len(slice))
	var err error
	for i, s := range slice {
		result[i], err = common.GetBigIntFromStr(s)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

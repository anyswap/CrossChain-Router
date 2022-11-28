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
		switch {
		case len(erc20SwapInfo.Path) > 0:
			swapinfo.ERC20SwapInfo = &ERC20SwapInfo{
				Token:         erc20SwapInfo.Token,
				TokenID:       erc20SwapInfo.TokenID,
				ForNative:     erc20SwapInfo.ForNative,
				ForUnderlying: erc20SwapInfo.ForUnderlying,
				Path:          erc20SwapInfo.Path,
			}
			if erc20SwapInfo.AmountOutMin != nil {
				swapinfo.ERC20SwapInfo.AmountOutMin = erc20SwapInfo.AmountOutMin.String()
			}
		case erc20SwapInfo.CallProxy != "":
			swapinfo.ERC20SwapInfo = &ERC20SwapInfo{
				Token:     erc20SwapInfo.Token,
				TokenID:   erc20SwapInfo.TokenID,
				SwapoutID: erc20SwapInfo.SwapoutID,
				CallProxy: erc20SwapInfo.CallProxy,
			}
			if erc20SwapInfo.CallData != nil {
				swapinfo.ERC20SwapInfo.CallData = erc20SwapInfo.CallData.String()
			}
		default:
			swapinfo.ERC20SwapInfo = &ERC20SwapInfo{
				Token:     erc20SwapInfo.Token,
				TokenID:   erc20SwapInfo.TokenID,
				SwapoutID: erc20SwapInfo.SwapoutID,
			}
		}
	case info.NFTSwapInfo != nil:
		nftSwapInfo := info.NFTSwapInfo
		swapinfo.NFTSwapInfo = &NFTSwapInfo{
			Token:   nftSwapInfo.Token,
			TokenID: nftSwapInfo.TokenID,
			IDs:     fromBigIntSlice(nftSwapInfo.IDs),
			Amounts: fromBigIntSlice(nftSwapInfo.Amounts),
			Batch:   nftSwapInfo.Batch,
			Data:    nftSwapInfo.Data.String(),
		}
	case info.AnyCallSwapInfo != nil:
		anycallSwapInfo := info.AnyCallSwapInfo
		swapinfo.AnyCallSwapInfo = &AnyCallSwapInfo{
			CallFrom: anycallSwapInfo.CallFrom,
			CallTo:   anycallSwapInfo.CallTo,
			CallData: common.ToHex(anycallSwapInfo.CallData),
			Fallback: anycallSwapInfo.Fallback,
			Flags:    anycallSwapInfo.Flags,
			AppID:    anycallSwapInfo.AppID,
			Nonce:    anycallSwapInfo.Nonce,
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
		switch {
		case len(erc20SwapInfo.Path) > 0:
			amountOutMin, err := common.GetBigIntFromStr(erc20SwapInfo.AmountOutMin)
			if err != nil {
				return info, fmt.Errorf("wrong amountOutMin %v", erc20SwapInfo.AmountOutMin)
			}
			info.ERC20SwapInfo = &tokens.ERC20SwapInfo{
				Token:         erc20SwapInfo.Token,
				TokenID:       erc20SwapInfo.TokenID,
				ForNative:     erc20SwapInfo.ForNative,
				ForUnderlying: erc20SwapInfo.ForUnderlying,
				Path:          erc20SwapInfo.Path,
				AmountOutMin:  amountOutMin,
			}
		case erc20SwapInfo.CallProxy != "":
			info.ERC20SwapInfo = &tokens.ERC20SwapInfo{
				Token:     erc20SwapInfo.Token,
				TokenID:   erc20SwapInfo.TokenID,
				SwapoutID: erc20SwapInfo.SwapoutID,
				CallProxy: erc20SwapInfo.CallProxy,
				CallData:  common.FromHex(erc20SwapInfo.CallData),
			}
		default:
			info.ERC20SwapInfo = &tokens.ERC20SwapInfo{
				Token:     erc20SwapInfo.Token,
				TokenID:   erc20SwapInfo.TokenID,
				SwapoutID: erc20SwapInfo.SwapoutID,
			}
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
			Data:    hexutil.Bytes(nftSwapInfo.Data),
		}
	case swapinfo.AnyCallSwapInfo != nil:
		anyCallSwapInfo := swapinfo.AnyCallSwapInfo
		info.AnyCallSwapInfo = &tokens.AnyCallSwapInfo{
			CallFrom: anyCallSwapInfo.CallFrom,
			CallTo:   anyCallSwapInfo.CallTo,
			CallData: common.FromHex(anyCallSwapInfo.CallData),
			Fallback: anyCallSwapInfo.Fallback,
			Flags:    anyCallSwapInfo.Flags,
			AppID:    anyCallSwapInfo.AppID,
			Nonce:    anyCallSwapInfo.Nonce,
		}
	}
	return info, nil
}

func fromBigIntSlice(slice []*big.Int) []string {
	result := make([]string, len(slice))
	for i, elem := range slice {
		result[i] = elem.String()
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

package mongodb

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/common/hexutil"
	"github.com/anyswap/CrossChain-Router/tokens"
)

// ConvertToSwapInfo convert
func ConvertToSwapInfo(info *tokens.SwapInfo) SwapInfo {
	swapinfo := SwapInfo{}
	if info.RouterSwapInfo != nil {
		swapinfo.RouterSwapInfo = &RouterSwapInfo{
			ForNative:     info.ForNative,
			ForUnderlying: info.ForUnderlying,
			Token:         info.Token,
			TokenID:       info.TokenID,
			Path:          info.Path,
		}
		if info.AmountOutMin != nil {
			swapinfo.AmountOutMin = info.AmountOutMin.String()
		}
	}
	if info.AnyCallSwapInfo != nil {
		swapinfo.AnyCallSwapInfo = &AnyCallSwapInfo{
			CallFrom:   info.CallFrom,
			CallTo:     info.CallTo,
			CallData:   fromHexBytesSlice(info.CallData),
			Callbacks:  info.Callbacks,
			CallNonces: fromBigIntSlice(info.CallNonces),
		}
	}
	return swapinfo
}

// ConvertFromSwapInfo convert
func ConvertFromSwapInfo(swapinfo *SwapInfo) (tokens.SwapInfo, error) {
	info := tokens.SwapInfo{}
	if swapinfo.RouterSwapInfo != nil {
		var amountOutMin *big.Int
		var err error
		if len(swapinfo.Path) > 0 {
			amountOutMin, err = common.GetBigIntFromStr(swapinfo.AmountOutMin)
			if err != nil {
				return info, fmt.Errorf("wrong amountOutMin %v", swapinfo.AmountOutMin)
			}
		}
		info.RouterSwapInfo = &tokens.RouterSwapInfo{
			ForNative:     swapinfo.ForNative,
			ForUnderlying: swapinfo.ForUnderlying,
			Token:         swapinfo.Token,
			TokenID:       swapinfo.TokenID,
			Path:          swapinfo.Path,
			AmountOutMin:  amountOutMin,
		}
	}
	if swapinfo.AnyCallSwapInfo != nil {
		nonces, err := toBigIntSlice(swapinfo.CallNonces)
		if err != nil {
			return info, fmt.Errorf("wrong nonces %v", swapinfo.CallNonces)
		}
		info.AnyCallSwapInfo = &tokens.AnyCallSwapInfo{
			CallFrom:   swapinfo.CallFrom,
			CallTo:     swapinfo.CallTo,
			CallData:   toHexBytesSlice(swapinfo.CallData),
			Callbacks:  swapinfo.Callbacks,
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

package cosmos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func ParseCoinsNormalized(coinStr string) (sdk.Coins, error) {
	return sdk.ParseCoinsNormalized(coinStr)
}

func ParseCoinsFee(amount string) (sdk.Coins, error) {
	if parsedFees, err := sdk.ParseCoinsNormalized(amount); err != nil {
		return nil, err
	} else {
		return parsedFees, nil
	}
}

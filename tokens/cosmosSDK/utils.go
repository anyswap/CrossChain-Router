package cosmosSDK

import (
	"github.com/cosmos/cosmos-sdk/types"
)

func ParseCoinsNormalized(coinStr string) (types.Coins, error) {
	return types.ParseCoinsNormalized(coinStr)
}

func ParseCoinsFee(amount string) (types.Coins, error) {
	if parsedFees, err := types.ParseCoinsNormalized(amount); err != nil {
		return nil, err
	} else {
		return parsedFees, nil
	}
}

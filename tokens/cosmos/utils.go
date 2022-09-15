package cosmos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func ParseCoinsNormalized(coinStr string) (sdk.Coins, error) {
	return sdk.ParseCoinsNormalized(coinStr)
}

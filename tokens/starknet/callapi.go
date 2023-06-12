package starknet

import (
	"fmt"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"math/big"

	"github.com/dontpanicdao/caigo/types"
)

const StarkGateETH = "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"

func (b *Bridge) GetBalance(account string) (*big.Int, error) {
	call := types.FunctionCall{
		ContractAddress:    types.HexToHash(StarkGateETH),
		EntryPointSelector: "balanceOf",
		Calldata:           []string{account},
	}
	ret, err := b.provider.Call(call)
	if err != nil {
		return nil, err
	}
	if balance, ok := new(big.Int).SetString(ret[0], 0); ok {
		return balance, nil
	}

	return nil, fmt.Errorf("get balance parse failed, call returned: %s", ret)
}

func (b *Bridge) EstimateFee(call interface{}) (*big.Int, error) {
	estimate, err := b.provider.EstimateFee(call)
	if err != nil {
		return nil, err
	}
	fee, ok := big.NewInt(0).SetString(string(estimate.OverallFee), 0)
	if !ok {
		return nil, tokens.ErrMatchFee
	}
	invokeMaxFee := fee.Mul(fee, big.NewInt(3))

	return invokeMaxFee, nil
}

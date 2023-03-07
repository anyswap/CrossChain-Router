package base

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

type ReSwapableBridgeBase struct {
	reswapMaxValue uint64
	txTimeout      uint64
}

// NewNonceSetterBase new base nonce setter
func NewReSwapableBridgeBase() *ReSwapableBridgeBase {
	return &ReSwapableBridgeBase{
		reswapMaxValue: uint64(1),
		txTimeout:      uint64(60 * 10),
	}
}

func (b *ReSwapableBridgeBase) SetReswapMaxValueRate(rate uint64) {
	b.reswapMaxValue = rate
}

func (b *ReSwapableBridgeBase) SetTimeoutConfig(txTimeout uint64) {
	b.txTimeout = txTimeout
}

func (b *ReSwapableBridgeBase) GetTimeoutConfig() uint64 {
	return b.txTimeout
}

func (b *ReSwapableBridgeBase) SetTxTimeout(args *tokens.BuildTxArgs, txTimeout *uint64) {
	if args.ERC20SwapInfo == nil {
		return
	}
	if args.Extra.TTL != nil {
		return
	}
	swapInfo := args.ERC20SwapInfo
	tokenID := swapInfo.TokenID
	if params.IsInBigValueWhitelist(tokenID, args.From) ||
		params.IsInBigValueWhitelist(tokenID, args.To) {
		args.Extra.TTL = txTimeout
		return
	}
	bridge := router.GetBridgeByChainID(args.FromChainID.String())
	if bridge == nil {
		return
	}
	tokenCfg := bridge.GetTokenConfig(swapInfo.Token)
	if tokenCfg == nil {
		return
	}
	fromDecimals := tokenCfg.Decimals
	bigValueThreshold := tokens.GetBigValueThreshold(tokenID, args.FromChainID.String(), args.ToChainID.String(), fromDecimals)
	bigValueThreshold.Mul(bigValueThreshold, new(big.Int).SetUint64(b.reswapMaxValue))
	bigValueThreshold.Div(bigValueThreshold, big.NewInt(1000))
	if args.SwapValue.Cmp(bigValueThreshold) <= 0 {
		args.Extra.TTL = txTimeout
	}
}

func (b *ReSwapableBridgeBase) IsTxTimeout(txValue, currentValue *uint64) bool {
	return txValue != nil && *txValue > 0 && *currentValue > *txValue
}

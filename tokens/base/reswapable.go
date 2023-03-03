package base

import "github.com/anyswap/CrossChain-Router/v3/tokens"

type ReSwapableBridgeBase struct {
}

func (b *ReSwapableBridgeBase) SetTxTimeout(args *tokens.BuildTxArgs, txTimeout *uint64) {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	if args.Extra.TTL == nil {
		args.Extra.TTL = txTimeout
	}
}

func (b *ReSwapableBridgeBase) IsTxTimeout(txValue, currentValue *uint64) bool {
	return *txValue > 0 && *currentValue > *txValue
}

package ripple

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// RegisterSwap api
func (b *Bridge) RegisterSwap(txHash string, args *tokens.RegisterArgs) ([]*tokens.SwapTxInfo, []error) {
	swapType := args.SwapType
	logIndex := args.LogIndex

	switch swapType {
	case tokens.ERC20SwapType:
		return b.registerERC20SwapTx(txHash, logIndex)
	case tokens.GasSwapType:
		return b.registerGasSwapTx(txHash, logIndex)
	default:
		return nil, []error{tokens.ErrSwapTypeNotSupported}
	}
}

func (b *Bridge) registerERC20SwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	swapInfo, err := b.verifySwapoutTx(txHash, logIndex, true)
	return []*tokens.SwapTxInfo{swapInfo}, []error{err}
}

package aptos

import (
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func (b *Bridge) registerGasSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	return []*tokens.SwapTxInfo{}, []error{tokens.ErrNotSupportFromAptos}
}

func (b *Bridge) buildGasSwapTxArg(args *tokens.BuildTxArgs) (err error) {

	input := []byte(args.SwapID)
	args.Input = (*hexutil.Bytes)(&input)
	if !b.IsValidAddress(args.Bind) {
		return tokens.ErrWrongBindAddress
	}
	args.To = args.Bind // to
	sendValue, err := tokens.CheckGasSwapValue(args.FromChainID, args.GasSwapInfo, args.OriginValue)
	if err != nil {
		return err
	}
	args.Value = sendValue
	return nil
}

func (b *Bridge) verifyGasSwapTx(txHash string, _ int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	return nil, tokens.ErrSwapTypeNotSupported
}

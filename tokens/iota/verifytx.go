package iota

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
)

var errTxResultType = errors.New("tx type is not data.TxResult")

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	return
}

// VerifyTransaction impl
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifySwapoutTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

//nolint:gocyclo,funlen // ok
func (b *Bridge) verifySwapoutTx(txHash string, _ int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	return nil, nil
}

func (b *Bridge) checkToken(token *tokens.TokenConfig, txmeta *data.TransactionWithMetaData) error {
	return nil
}

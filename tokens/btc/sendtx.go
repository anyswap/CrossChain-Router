package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	return txHash, tokens.ErrNotImplemented
}

func (b *Bridge) BroadcastTxCommit(signedTx interface{}) (txHash string, err error) {
	return txHash, tokens.ErrNotImplemented
}

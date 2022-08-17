package cardano

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	return "", tokens.ErrNotImplemented
}

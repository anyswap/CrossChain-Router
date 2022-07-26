package iota

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	iotago "github.com/iotaledger/iota.go/v2"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	message := signedTx.(*iotago.Message)
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	for _, url := range urls {
		if txHash, err := CommitMessage(url, message); err == nil {
			return txHash, nil
		}
	}
	return txHash, tokens.ErrCommitMessage
}

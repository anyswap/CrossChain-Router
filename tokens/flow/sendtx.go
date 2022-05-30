package flow

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	sdk "github.com/onflow/flow-go-sdk"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	tx := signedTx.(*sdk.Transaction)
	for _, url := range urls {
		result, err := sendTransaction(url, tx)
		if err == nil {
			return result, nil
		}
	}
	return "", tokens.ErrSendTx
}

package near

import (
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/near/borsh-go"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	signTx := signedTx.(*SignedTransaction)
	buf, err := borsh.Serialize(*signTx)
	if err != nil {
		return "", err
	}
	txHash, err = b.BroadcastTxCommit(buf)
	if err != nil {
		return "", err
	} else if !params.IsParallelSwapEnabled() {
		sender := signTx.Transaction.SignerID
		nonce := signTx.Transaction.Nonce
		b.SetNonce(sender, nonce+1)
	}
	return txHash, nil
}

// BroadcastTxCommit broadcast tx
func (b *Bridge) BroadcastTxCommit(signedTx []byte) (result string, err error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	var success bool
	for _, url := range urls {
		result, err = BroadcastTxCommit(url, signedTx)
		if err == nil {
			success = true
		}
	}
	if success {
		return result, nil
	}
	return "", tokens.ErrBroadcastTx
}

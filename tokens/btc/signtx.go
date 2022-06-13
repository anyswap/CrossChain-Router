package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	return nil, txHash, tokens.ErrNotImplemented
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key string
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	return signTransaction(rawTx, "")
}

func signTransaction(tx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	return nil, "", tokens.ErrNotImplemented
}

package flow

import (
	"crypto/ed25519"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	return nil, "", nil
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key string
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	return nil, "", nil
}

func (b *Bridge) verifyTransactionReceiver(tx *interface{}, tokenID string) error {
	return nil
}

func signTransaction(tx *interface{}, privKey *ed25519.PrivateKey) (signedTx *interface{}, txHash string, err error) {
	return nil, "", nil
}

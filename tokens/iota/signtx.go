package iota

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
)

func (b *Bridge) verifyTransactionWithArgs(tx data.Transaction, args *tokens.BuildTxArgs) error {
	return nil
}

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	return
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signTx interface{}, txHash string, err error) {
	return
}

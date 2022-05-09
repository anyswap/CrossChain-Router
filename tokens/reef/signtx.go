package reef

import (
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/anyswap/CrossChain-Router/v3/types"
	schnorrkel "github.com/ChainSafe/go-schnorrkel"
	"github.com/gtank/merlin"
)

type ReefSignedTx {
	*types.Transaction
	Signature *schnorrkel.Signature
}

// SignTransactionWithPrivateKey sign tx with private key (use for testing)
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey *schnorrkel.SecretKey) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return nil, "", errors.New("wrong raw tx param")
	}

	// TODO
	var hash []byte
	t := NewTranscript(applabel)
	t.AppendMessage(label, msg)
	sig, err := privKey.Sign(t)
	if err != nil {
		return nil, "", err
	}

	signedTx := &ReefSignedTx{
		tx,
		sig,
	}

	// TODO
	//txHash = signedTx.Hash().String()
	txHash := ""
	log.Info(b.ChainConfig.BlockChain+" SignTransaction success", "txhash", txHash, "nonce", signedTx.Nonce())
	return signedTx, txHash, err
}
package aptos

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/log"
)

// SendTransaction impl
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(*Transaction)
	if !ok {
		return "", errors.New("wrong signed transaction type")
	}
	txInfo, err := b.Client.SubmitTranscation(tx)
	if err != nil {
		log.Info("Solana SendTransaction failed", "err", err)
	} else {
		log.Info("Solana SendTransaction success", "hash", txInfo.Hash)

	}
	return txInfo.Hash, err
}

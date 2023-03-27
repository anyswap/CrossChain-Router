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
	txInfo, err := b.SubmitTranscation(tx)
	if err != nil {
		log.Info("Aptos SendTransaction failed", "err", err)
		return "", err
	} else {
		log.Info("Aptos SendTransaction success", "hash", txInfo.Hash)
	}
	return txInfo.Hash, err
}

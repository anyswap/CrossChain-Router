package aptos

import (
	"errors"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
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

		if !params.IsParallelSwapEnabled() {
			sequence, _ := strconv.ParseUint(tx.SequenceNumber, 10, 64)
			b.SetNonce(tx.Sender, sequence+1)
		}
	}
	return txInfo.Hash, err
}

package reef

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/log"
)

func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(*ReefTransaction)
	if !ok {
		log.Printf("signed tx is %+v", signedTx)
		return "", errors.New("wrong signed transaction type")
	}

	txHash, err = SendSignedTx(tx.buildScriptParam())
	if err != nil {
		log.Warn("SendTransaction failed", "chainID", b.ChainConfig.ChainID, "err", err)
		return "", err
	}
	log.Info("SendTransaction success", "chainID", b.ChainConfig.ChainID, "hash", txHash)
	return txHash, nil
}

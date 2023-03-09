package reef

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(*ReefTransaction)
	if !ok {
		log.Printf("signed tx is %+v", signedTx)
		return "", errors.New("wrong signed transaction type")
	}

	txHash, err = SendSignedTx(tx.buildScriptParam())
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(*tx.TxHash, txHash) {
		return "", fmt.Errorf("txhash dismatch txHash %s sendTx %s", *tx.TxHash, txHash)
	}
	return txHash, nil
}

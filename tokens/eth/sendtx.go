package eth

import (
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/types"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(*types.Transaction)
	if !ok {
		fmt.Printf("signed tx is %+v\n", signedTx)
		return "", errors.New("wrong signed transaction type")
	}
	sender, err := types.Sender(b.Signer, tx)
	if err != nil {
		return txHash, err
	}
	err = b.checkBalance(sender.String(), tx.Value(), tx.GasPrice(), tx.Gas(), false)
	if err != nil {
		return txHash, err
	}
	txHash, err = b.SendSignedTransaction(tx)
	if err != nil {
		log.Info("SendTransaction failed", "hash", txHash, "err", err)
		return txHash, err
	}
	log.Info("SendTransaction success", "hash", txHash)
	//#log.Trace("SendTransaction success", "raw", tx.RawStr())
	return txHash, nil
}

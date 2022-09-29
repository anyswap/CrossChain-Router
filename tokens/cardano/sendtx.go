package cardano

import (
	"fmt"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	signedTransaction := signedTx.(*SignedTransaction)
	cmdString := fmt.Sprintf(SubmitCmd, signedTransaction.FilePath)
	if _, err := ExecCmd(cmdString, " "); err != nil {
		return "", err
	} else {
		return signedTransaction.TxHash, nil
	}
}

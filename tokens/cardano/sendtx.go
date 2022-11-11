package cardano

import (
	"fmt"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (string, error) {
	signedTransaction := signedTx.(*SignedTransaction)
	cmdString := fmt.Sprintf(SubmitCmd, signedTransaction.FilePath)
	if _, err := ExecCmd(cmdString, " "); err != nil {
		return "", err
	} else {
		TransactionChaining.InputKey.TxHash = signedTransaction.TxHash
		TransactionChaining.InputKey.TxIndex = signedTransaction.TxIndex
		TransactionChaining.AssetsMap = signedTransaction.AssetsMap
		AddTransactionChainingKeyCache(signedTransaction.TxHash, &signedTransaction.TxIns)
		return signedTransaction.TxHash, nil
	}
}

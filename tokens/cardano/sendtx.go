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
		txIns := signedTransaction.TxIns
		for _, inputKey := range txIns {
			TransactionChainingKeyCache.SpentUtxoList = append(TransactionChainingKeyCache.SpentUtxoList, inputKey)
			TransactionChainingKeyCache.SpentUtxoMap[inputKey] = true
		}
		cacheListLength := len(TransactionChainingKeyCache.SpentUtxoList)
		if cacheListLength > 100 {
			TransactionChainingKeyCache.SpentUtxoList = TransactionChainingKeyCache.SpentUtxoList[cacheListLength-100 : cacheListLength]
		}
		return signedTransaction.TxHash, nil
	}
}

package cardano

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

var (
	SubmitCmd = "cardano-cli transaction submit --testnet-magic 1097911063 --tx-file %s"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	signedTransaction := signedTx.(*SignedTransaction)
	cmdString := fmt.Sprintf(SubmitCmd, signedTransaction.FilePath)
	list := strings.Split(cmdString, " ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return signedTransaction.TxHash, nil
}

package reef

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
)

func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	return "", nil
}

// SendTransaction send signed tx
func (b *Bridge) SendExtrinsic(signedTx interface{}) (txHash, txKey string, err error) {
	tx, ok := signedTx.(*Extrinsic)
	if !ok {
		log.Printf("signed tx is %+v", signedTx)
		return "", "", errors.New("wrong signed transaction type")
	}
	txHash, txKey, err = b.SendSignedTransaction(tx)
	if err != nil {
		log.Info("SendExtrinsic failed", "hash", txHash, "txkey", txKey, "err", err)
	} else {
		log.Info("SendExtrinsic success", "hash", txHash)
	}
	if params.IsDebugMode() {
		log.Infof("SendExtrinsic rawtx is %v", signedTx)
	}
	return txHash, txKey, err
}

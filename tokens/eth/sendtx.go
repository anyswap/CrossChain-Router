package eth

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(*types.Transaction)
	if !ok {
		log.Printf("signed tx is %+v", signedTx)
		return "", errors.New("wrong signed transaction type")
	}
	chainID := b.ChainConfig.ChainID
	txHash, err = b.SendSignedTransaction(tx)
	if err != nil {
		log.Info("SendTransaction failed", "chainID", chainID, "hash", txHash, "err", err)
	} else {
		log.Info("SendTransaction success", "chainID", chainID, "hash", txHash)
		sender, errt := types.Sender(b.Signer, tx)
		if errt != nil {
			log.Error("SendTransaction get sender failed", "chainID", chainID, "tx", txHash, "err", errt)
			return txHash, errt
		}
		b.SetNonce(sender.LowerHex(), tx.Nonce()+1)
	}
	if params.IsDebugMode() {
		log.Infof("SendTransaction on chain %v, rawtx is %v", chainID, tx.RawStr())
	}
	return txHash, err
}

package eth

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

func (b *Bridge) SendZKSyncTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(*SignedZKSyncTx)
	if !ok {
		log.Printf("signed tx is %+v", signedTx)
		return "", errors.New("wrong signed transaction type")
	}
	txHash, err = b.SendSignedZKSyncTransaction(tx.Raw)
	chainID := b.ChainConfig.ChainID
	if err != nil {
		log.Info("SendTransaction failed", "chainID", chainID, "hash", txHash, "err", err)
	} else {
		log.Info("SendTransaction success", "chainID", chainID, "hash", txHash)
	}
	return txHash, err
}

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	if b.IsZKSync() {
		return b.SendZKSyncTransaction(signedTx)
	}
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
	}
	if params.IsDebugMode() {
		log.Infof("SendTransaction on chain %v, rawtx is %v", chainID, tx.RawStr())
	}
	return txHash, err
}

package flow

import (
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	sdk "github.com/onflow/flow-go-sdk"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx := signedTx.(*sdk.Transaction)
	txHash, err = b.BroadcastTxCommit(tx)
	if err != nil {
		log.Warn("SendTransaction failed", "err", err)
		return "", err
	}
	log.Info("SendTransaction success", "hash", txHash)
	if !params.IsParallelSwapEnabled() {
		sender := tx.Payer
		nonce := tx.ProposalKey.SequenceNumber
		b.SetNonce(sender.Hex(), nonce+1)
	}
	return txHash, nil
}

func (b *Bridge) BroadcastTxCommit(signedTx *sdk.Transaction) (txHash string, err error) {
	urls := b.GatewayConfig.AllGatewayURLs
	var success bool
	for _, url := range urls {
		txHash, err = sendTransaction(url, signedTx)
		if err == nil {
			success = true
		} else {
			log.Warn("BroadcastTxCommit failed", "err", err)
		}
	}
	if success {
		return txHash, nil
	}
	return "", tokens.ErrSendTx
}

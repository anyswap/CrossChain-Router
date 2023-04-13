package btc

import (
	"bytes"
	"encoding/hex"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	authoredTx, ok := signedTx.(*txauthor.AuthoredTx)
	if !ok {
		return "", tokens.ErrWrongRawTx
	}

	tx := authoredTx.Tx
	if tx == nil {
		return "", tokens.ErrWrongRawTx
	}

	buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
	err = tx.Serialize(buf)
	if err != nil {
		return "", err
	}
	txHex := hex.EncodeToString(buf.Bytes())

	return b.BroadcastTxCommit(txHex)
}

// PostTransaction impl
func (b *Bridge) BroadcastTxCommit(txHex string) (txHash string, err error) {
	urls := b.GatewayConfig.AllGatewayURLs
	var success bool
	for _, url := range urls {
		txHash, err = PostTransaction(url, txHex)
		if err == nil {
			success = true
		}
	}
	if success {
		return txHash, nil
	}
	return "", errors.New("PostTransaction error")
}

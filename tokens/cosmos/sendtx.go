package cosmos

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (string, error) {
	if txBytes, ok := signedTx.([]byte); !ok {
		return "", errors.New("wrong signed transaction type")
	} else {
		req := &BroadcastTxRequest{
			TxBytes: string(txBytes),
			Mode:    "BROADCAST_MODE_SYNC",
		}
		if txRes, err := b.BroadcastTx(req); err != nil {
			return "", err
		} else {
			if txRes == "" {
				return "", tokens.ErrBroadcastTx
			}
			var txResponse *BroadcastTxResponse
			if err := json.Unmarshal([]byte(txRes), &txResponse); err != nil {
				return "", err
			}
			if txResponse.TxResponse.Code != 0 && txResponse.TxResponse.Code != 19 {
				return "", fmt.Errorf("SendTransaction error, code: %v", txResponse.TxResponse.Code)
			}
			return txResponse.TxResponse.TxHash, nil
		}
	}
}

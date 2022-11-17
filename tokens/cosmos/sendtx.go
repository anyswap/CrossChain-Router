package cosmos

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	BroadTx = "/cosmos/tx/v1beta1/txs"
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
			var txResponse *BroadcastTxResponse
			if err := json.Unmarshal([]byte(txRes), &txResponse); err != nil {
				return "", err
			}
			if txResponse.TxResponse.Code != 0 && txResponse.TxResponse.Code != 19 {
				return "", fmt.Errorf(
					"SendTransaction error, code: %v, log:%v",
					txResponse.TxResponse.Code, txResponse.TxResponse.RawLog)
			}
			return txResponse.TxResponse.TxHash, nil
		}
	}
}

func (b *Bridge) BroadcastTx(req *BroadcastTxRequest) (string, error) {
	if data, err := json.Marshal(req); err != nil {
		return "", err
	} else {
		for _, url := range b.AllGatewayURLs {
			restApi := url + BroadTx
			if res, err := client.RPCJsonPostWithTimeout(restApi, string(data), 120); err == nil {
				return res, nil
			}
		}
		return "", tokens.ErrBroadcastTx
	}
}

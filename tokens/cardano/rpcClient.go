package cardano

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	rpcTimeout = 60
)

func GetTransactionByHash(url, txHash string) (*Transaction, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(QueryTransaction, txHash)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result TransactionResult
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	}
	if len(result.Transactions) == 0 {
		return nil, tokens.ErrTxNotFound
	}
	return &result.Transactions[0], nil
}

func GetUtxosByAddress(url, address string) (*[]Output, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(QueryOutputs, address)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result OutputsResult
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	}
	if len(result.Outputs) == 0 {
		return nil, tokens.ErrOutputLength
	}
	return &result.Outputs, nil
}

func GetLatestBlockNumber() (uint64, error) {
	if res, err := queryTipCmd(); err != nil {
		return 0, err
	} else {
		return res.Slot, nil
	}
}

func queryTipCmd() (*Tip, error) {
	if execRes, err := ExecCmd(QueryTipCmd, " "); err != nil {
		return nil, err
	} else {
		var tip Tip
		if err := json.Unmarshal([]byte(execRes), &tip); err != nil {
			return nil, err
		}
		return &tip, nil
	}
}

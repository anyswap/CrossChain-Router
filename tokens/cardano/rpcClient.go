package cardano

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

const (
	rpcTimeout = 60
)

func GetTransactionByHash(url, txHash string) (*Transaction, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(QueryMethod, txHash)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result TransactionResult
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	} else {
		return &result.Transactions[0], nil
	}
}

func GetLatestBlockNumber() (uint64, error) {
	if res, err := queryTipCmd(); err != nil {
		return 0, err
	} else {
		return res.Block, nil
	}
}

func queryTipCmd() (*Tip, error) {
	if execRes, err := ExecCmd(QueryTipCmd, " "); err != nil {
		return nil, err
	} else {
		var tip Tip
		if err := json.Unmarshal([]byte(execRes), &tip); err != nil {
			return nil, err
		} else {
			return &tip, nil
		}
	}
}

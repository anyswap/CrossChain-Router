package flow

import (
	"context"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access/http"
)

var (
	rpcTimeout = 60
	ctx        = context.Background()
)

// SetRPCTimeout set rpc timeout
func SetRPCTimeout(timeout int) {
	rpcTimeout = timeout
}

func GetBlockNumberByHash(url, hash string) (uint64, error) {
	path := "/blocks/" + hash
	var result sdk.Block
	err := client.RPCGetWithTimeout(&result, joinURLPath(url, path), rpcTimeout)
	if err != nil {
		return 0, err
	}
	return result.Height, nil
}

// GetLatestBlockNumber get latest block height
func GetLatestBlockNumber(url string) (uint64, error) {
	flowClient, err1 := http.NewClient(url)
	if err1 != nil {
		return 0, err1
	}
	latestBlock, err2 := flowClient.GetLatestBlock(ctx, true)
	if err2 != nil {
		return 0, err2
	}
	return latestBlock.Height, nil
}

// GetLatestBlockNumber get latest block height
func GetAccountNonce(url, account string) (uint64, error) {
	path := "/accounts/" + account
	var result sdk.Account
	err := client.RPCGetWithTimeout(&result, joinURLPath(url, path), rpcTimeout)
	if err != nil {
		return 0, err
	}
	return result.Keys[0].SequenceNumber, nil
}

func BroadcastTxCommit(url string, signedTx []byte) (string, error) {
	return "", nil
}

// GetTransactionByHash get tx by hash
func GetTransactionByHash(url, txHash string) (*sdk.TransactionResult, error) {
	path := "/transaction_results/" + txHash
	var result sdk.TransactionResult
	err := client.RPCGetWithTimeout(&result, joinURLPath(url, path), rpcTimeout)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func joinURLPath(url, path string) string {
	url = strings.TrimSuffix(url, "/")
	if !strings.HasPrefix(path, "/") {
		url += "/"
	}
	return url + path
}

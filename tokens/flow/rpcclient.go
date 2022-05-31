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
func GetLatestBlock(url string) (*sdk.Block, error) {
	flowClient, err1 := http.NewClient(url)
	if err1 != nil {
		return nil, err1
	}
	latestBlock, err2 := flowClient.GetLatestBlock(ctx, true)
	if err2 != nil {
		return nil, err2
	}
	return latestBlock, nil
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

func sendTransaction(url string, signedTx *sdk.Transaction) (string, error) {
	flowClient, err1 := http.NewClient(url)
	if err1 != nil {
		return "", err1
	}
	err2 := flowClient.SendTransaction(ctx, *signedTx)
	if err2 != nil {
		return "", err2
	}
	return signedTx.ID().String(), nil
}

// GetTransactionByHash get tx by hash
func GetTransactionByHash(url, txHash string) (*sdk.TransactionResult, error) {
	flowClient, err1 := http.NewClient(url)
	if err1 != nil {
		return nil, err1
	}
	result, err2 := flowClient.GetTransactionResult(ctx, sdk.HexToID(txHash))
	if err2 != nil {
		return nil, err2
	}
	return result, nil
}

func joinURLPath(url, path string) string {
	url = strings.TrimSuffix(url, "/")
	if !strings.HasPrefix(path, "/") {
		url += "/"
	}
	return url + path
}

package flow

import (
	"context"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access/http"
)

var (
	ctx = context.Background()
)

func GetBlockNumberByHash(url string, blockId sdk.Identifier) (uint64, error) {
	flowClient, err1 := http.NewClient(url)
	if err1 != nil {
		return 0, err1
	}
	latestBlock, err2 := flowClient.GetBlockByID(ctx, blockId)
	if err2 != nil {
		return 0, err2
	}
	return latestBlock.Height, nil
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
func GetAccount(url, address string) (*sdk.Account, error) {
	flowClient, err1 := http.NewClient(url)
	if err1 != nil {
		return nil, err1
	}
	account, err2 := flowClient.GetAccount(ctx, sdk.HexToAddress(address))
	if err2 != nil {
		return nil, err2
	}
	return account, nil
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

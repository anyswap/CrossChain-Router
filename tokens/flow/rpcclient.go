package flow

import (
	"context"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access/grpc"
)

var (
	ctx = context.Background()
)

func GetBlockNumberByHash(url string, blockId sdk.Identifier) (uint64, error) {
	flowClient, err := grpc.NewClient(url)
	if err != nil {
		return 0, err
	}
	latestBlock, err := flowClient.GetBlockByID(ctx, blockId)
	if err != nil {
		return 0, err
	}
	return latestBlock.Height, nil
}

// GetLatestBlockNumber get latest block height
func GetLatestBlock(url string) (*sdk.Block, error) {
	flowClient, err := grpc.NewClient(url)
	if err != nil {
		return nil, err
	}
	latestBlock, err := flowClient.GetLatestBlock(ctx, true)
	if err != nil {
		return nil, err
	}
	return latestBlock, nil
}

// GetLatestBlockNumber get latest block height
func GetAccount(url, address string) (*sdk.Account, error) {
	flowClient, err := grpc.NewClient(url)
	if err != nil {
		return nil, err
	}
	account, err := flowClient.GetAccount(ctx, sdk.HexToAddress(address))
	if err != nil {
		return nil, err
	}
	return account, nil
}

func sendTransaction(url string, signedTx *sdk.Transaction) (string, error) {
	flowClient, err := grpc.NewClient(url)
	if err != nil {
		return "", err
	}
	err = flowClient.SendTransaction(ctx, *signedTx)
	if err != nil {
		return "", err
	}
	return signedTx.ID().String(), nil
}

// GetTransactionByHash get tx by hash
func GetTransactionByHash(url, txHash string) (*sdk.TransactionResult, error) {
	flowClient, err := grpc.NewClient(url)
	if err != nil {
		return nil, err
	}
	result, err := flowClient.GetTransactionResult(ctx, sdk.HexToID(txHash))
	if err != nil {
		return nil, err
	}
	return result, nil
}

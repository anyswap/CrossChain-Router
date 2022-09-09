package cosmos

import (
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/cosmos/cosmos-sdk/api/tendermint/types"
)

const (
	LatestBlock = "/cosmos/base/tendermint/v1beta1/blocks/latest"
	TxByHash    = "/cosmos/tx/v1beta1/txs/"
	AccountInfo = "/cosmos/auth/v1beta1/accounts/"
)

func (c *CosmosRestClient) GetLatestBlockNumber(apiAddress string) (uint64, error) {
	var result types.Block
	if apiAddress == "" {
		restApi := apiAddress + LatestBlock
		if err := client.RPCGet(&result, restApi); err == nil {
			return uint64(result.Header.Height), nil
		} else {
			return 0, err
		}
	}
	for _, url := range c.BaseUrls {
		restApi := url + LatestBlock
		if err := client.RPCGet(&result, restApi); err == nil {
			return uint64(result.Header.Height), nil
		}
	}
	return 0, tokens.ErrRPCQueryError
}

func (c *CosmosRestClient) GetTransactionByHash(txHash string) (*GetTxResponse, error) {
	var result *GetTxResponse
	for _, url := range c.BaseUrls {
		restApi := url + TxByHash + txHash
		if err := client.RPCGet(&result, restApi); err == nil {
			return result, nil
		}
	}
	return nil, tokens.ErrTxNotFound
}

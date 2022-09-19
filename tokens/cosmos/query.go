package cosmosSDK

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	tendermintTypes "github.com/cosmos/cosmos-sdk/api/tendermint/types"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	LatestBlock = "/cosmos/base/tendermint/v1beta1/blocks/latest"
	TxByHash    = "/cosmos/tx/v1beta1/txs/"
	AccountInfo = "/cosmos/auth/v1beta1/accounts/"
	AtomBalance = "/cosmos/bank/v1beta1/balances/%s/by_denom?denom=%s"
)

func (c *CosmosRestClient) GetLatestBlockNumber(apiAddress string) (uint64, error) {
	var result tendermintTypes.Block
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

func (c *CosmosRestClient) GetCoinBalance(account, denom string) (*big.Int, error) {
	var result *bankTypes.QueryBalanceResponse
	requestUrl := fmt.Sprintf(AtomBalance, account, denom)
	for _, url := range c.BaseUrls {
		restApi := url + requestUrl
		if err := client.RPCGet(&result, restApi); err == nil {
			return result.Balance.Amount.BigInt(), nil
		}
	}
	return nil, tokens.ErrRPCQueryError
}

func (c *CosmosRestClient) CheckCoinBalance(account, denom string, amount *big.Int) error {
	if balance, err := c.GetCoinBalance(account, denom); err != nil {
		return err
	} else {
		if balance.Cmp(amount) > 0 {
			return nil
		}
		return fmt.Errorf(
			"insufficient native balance, need: %v, have: %v",
			amount, amount)
	}
}

func (c *CosmosRestClient) GetBaseAccount(address string) (*QueryAccountResponse, error) {
	var result *QueryAccountResponse
	for _, url := range c.BaseUrls {
		restApi := url + AccountInfo + address
		if err := client.RPCGet(&result, restApi); err == nil {
			return result, nil
		}
	}
	return nil, tokens.ErrRPCQueryError
}

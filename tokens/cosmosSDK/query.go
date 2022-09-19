package cosmosSDK

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	LatestBlock = "/cosmos/base/tendermint/v1beta1/blocks/latest"
	TxByHash    = "/cosmos/tx/v1beta1/txs/"
	AccountInfo = "/cosmos/auth/v1beta1/accounts/"
	AtomBalance = "/cosmos/bank/v1beta1/balances/%s/by_denom?denom=%s"
)

func (c *CosmosRestClient) GetLatestBlockNumber() (uint64, error) {
	var result *GetLatestBlockResponse
	for _, url := range c.BaseUrls {
		restApi := url + LatestBlock
		if err := client.RPCGet(&result, restApi); err == nil {
			if height, err := strconv.ParseUint(result.Block.Header.Height, 10, 64); err == nil {
				return height, nil
			}
		}
	}
	return 0, tokens.ErrRPCQueryError
}

func GetLatestBlockNumberByApiUrl(apiAddress string) (uint64, error) {
	var result *GetLatestBlockResponse
	restApi := apiAddress + LatestBlock
	if err := client.RPCGet(&result, restApi); err == nil {
		return strconv.ParseUint(result.Block.Header.Height, 10, 64)
	} else {
		return 0, err
	}
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
			if result.Status == "ERROR" {
				return nil, errors.New(fmt.Sprintf("GetBaseAccount error:%v address:%v", result.Msg, address))
			} else {
				return result, nil
			}
		}
	}
	return nil, tokens.ErrRPCQueryError
}

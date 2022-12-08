package cosmos

import (
	"fmt"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	LatestBlock = "/cosmos/base/tendermint/v1beta1/blocks/latest"
	TxByHash    = "/cosmos/tx/v1beta1/txs/"
	AccountInfo = "/cosmos/auth/v1beta1/accounts/"
	Balances    = "/cosmos/bank/v1beta1/balances/"
)

func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	var result *GetLatestBlockResponse
	for _, url := range b.AllGatewayURLs {
		restApi := url + LatestBlock
		if err := client.RPCGet(&result, restApi); err == nil {
			if height, err := strconv.ParseUint(result.Block.Header.Height, 10, 64); err == nil {
				return height, nil
			}
		}
	}
	return 0, tokens.ErrRPCQueryError
}

func (b *Bridge) GetChainID() (string, error) {
	var result *GetLatestBlockResponse
	for _, url := range b.AllGatewayURLs {
		restApi := url + LatestBlock
		if err := client.RPCGet(&result, restApi); err == nil {
			return result.Block.Header.ChainID, nil
		}
	}
	return "", tokens.ErrRPCQueryError
}

func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	var result *GetLatestBlockResponse
	restApi := apiAddress + LatestBlock
	if err := client.RPCGet(&result, restApi); err == nil {
		return strconv.ParseUint(result.Block.Header.Height, 10, 64)
	} else {
		return 0, err
	}
}

func (b *Bridge) GetTransactionByHash(txHash string) (*GetTxResponse, error) {
	var result *GetTxResponse
	for _, url := range b.AllGatewayURLs {
		restApi := url + TxByHash + txHash
		if err := client.RPCGet(&result, restApi); err == nil {
			if result.Status == "ERROR" {
				return nil, fmt.Errorf(
					"GetTransactionByHash error, txHash: %v, msg: %v",
					txHash, result.Msg)
			} else {
				return result, nil
			}
		}
	}
	return nil, tokens.ErrTxNotFound
}

func (b *Bridge) GetBaseAccount(address string) (*QueryAccountResponse, error) {
	var result *QueryAccountResponse
	for _, url := range b.AllGatewayURLs {
		restApi := url + AccountInfo + address
		if err := client.RPCGet(&result, restApi); err == nil {
			if result.Status == "ERROR" {
				return nil, fmt.Errorf(
					"GetBaseAccount error, address: %v, msg: %v",
					address, result.Msg)
			} else {
				return result, nil
			}
		}
	}
	return nil, tokens.ErrRPCQueryError
}

func (b *Bridge) GetDenomBalance(address, denom string) (sdk.Int, error) {
	var result *QueryBalanceResponse
	for _, url := range b.AllGatewayURLs {
		restApi := url + Balances + address + "/" + denom
		if err := client.RPCGet(&result, restApi); err == nil {
			return result.Balance.Amount, nil
		}
	}
	return sdk.Int{}, tokens.ErrRPCQueryError
}

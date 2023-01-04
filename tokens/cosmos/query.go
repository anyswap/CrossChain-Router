package cosmos

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	LatestBlock = "/cosmos/base/tendermint/v1beta1/blocks/latest"
	TxByHash    = "/cosmos/tx/v1beta1/txs/"
	AccountInfo = "/cosmos/auth/v1beta1/accounts/"
	Balances    = "/cosmos/bank/v1beta1/balances/"
	SimulateTx  = "/cosmos/tx/v1beta1/simulate"
	BroadTx     = "/cosmos/tx/v1beta1/txs"
)

func joinURLPath(url, path string) string {
	url = strings.TrimSuffix(url, "/")
	if !strings.HasPrefix(path, "/") {
		url += "/"
	}
	return url + path
}

func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	var result *GetLatestBlockResponse
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, LatestBlock)
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
		restApi := joinURLPath(url, LatestBlock)
		if err := client.RPCGet(&result, restApi); err == nil {
			return result.Block.Header.ChainID, nil
		}
	}
	return "", tokens.ErrRPCQueryError
}

func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	var result *GetLatestBlockResponse
	restApi := joinURLPath(apiAddress, LatestBlock)
	if err := client.RPCGet(&result, restApi); err == nil {
		return strconv.ParseUint(result.Block.Header.Height, 10, 64)
	} else {
		return 0, err
	}
}

func (b *Bridge) GetTransactionByHash(txHash string) (*GetTxResponse, error) {
	var result *GetTxResponse
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, TxByHash+txHash)
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
		restApi := joinURLPath(url, AccountInfo+address)
		if err := client.RPCGet(&result, restApi); err == nil {
			if result.Status == "ERROR" {
				return nil, fmt.Errorf(
					"GetBaseAccount error, address: %v, msg: %v",
					address, result.Msg)
			} else {
				return result, nil
			}
		} else {
			log.Warn("GetBaseAccount failed", "url", restApi, "err", err)
		}
	}
	return nil, tokens.ErrRPCQueryError
}

func (b *Bridge) GetDenomBalance(address, denom string) (sdk.Int, error) {
	var result *QueryAllBalancesResponse
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, Balances+address)
		if err := client.RPCGet(&result, restApi); err == nil {
			for _, coin := range result.Balances {
				if coin.Denom == denom {
					return coin.Amount, nil
				}
			}
			return sdk.ZeroInt(), nil
		} else {
			log.Warn("GetDenomBalance failed", "url", restApi, "err", err)
		}
	}
	return sdk.ZeroInt(), tokens.ErrRPCQueryError
}

func (b *Bridge) SimulateTx(simulateReq *SimulateRequest) (string, error) {
	if data, err := json.Marshal(simulateReq); err != nil {
		return "", err
	} else {
		for _, url := range b.AllGatewayURLs {
			restApi := joinURLPath(url, SimulateTx)
			if res, err := client.RPCRawPostWithTimeout(restApi, string(data), 120); err == nil && res != "" && res != "\n" {
				return res, nil
			}
		}
		return "", tokens.ErrSimulateTx
	}
}

func (b *Bridge) BroadcastTx(req *BroadcastTxRequest) (string, error) {
	if data, err := json.Marshal(req); err != nil {
		return "", err
	} else {
		var res string
		var success bool
		for _, url := range b.AllGatewayURLs {
			restApi := joinURLPath(url, BroadTx)
			if res, err = client.RPCJsonPostWithTimeout(restApi, string(data), 120); err == nil {
				success = true
			}
		}
		if success {
			return res, nil
		}
		return "", tokens.ErrBroadcastTx
	}
}

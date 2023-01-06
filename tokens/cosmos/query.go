package cosmos

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
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

var wrapRPCQueryError = tokens.WrapRPCQueryError

func joinURLPath(url, path string) string {
	url = strings.TrimSuffix(url, "/")
	if !strings.HasPrefix(path, "/") {
		url += "/"
	}
	return url + path
}

func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	if result, err := b.GRPCGetLatestBlockNumber(); err == nil {
		return result, nil
	}
	var result *GetLatestBlockResponse
	var err error
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, LatestBlock)
		if err = client.RPCGet(&result, restApi); err == nil {
			if height, err := strconv.ParseUint(result.Block.Header.Height, 10, 64); err == nil {
				return height, nil
			}
		}
	}
	return 0, wrapRPCQueryError(err, "GetLatestBlockNumber")
}

func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	if result, err := b.GRPCGetLatestBlockNumberOf(apiAddress); err == nil {
		return result, nil
	}
	var result *GetLatestBlockResponse
	restApi := joinURLPath(apiAddress, LatestBlock)
	if err := client.RPCGet(&result, restApi); err == nil {
		return strconv.ParseUint(result.Block.Header.Height, 10, 64)
	} else {
		return 0, wrapRPCQueryError(err, "GetLatestBlockNumber")
	}
}

func (b *Bridge) GetChainID() (string, error) {
	if result, err := b.GRPCGetChainID(); err == nil {
		return result, nil
	}
	var result *GetLatestBlockResponse
	var err error
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, LatestBlock)
		if err = client.RPCGet(&result, restApi); err == nil {
			return result.Block.Header.ChainID, nil
		}
	}
	return "", wrapRPCQueryError(err, "GetChainID")
}

func (b *Bridge) GetTransactionByHash(txHash string) (*GetTxResponse, error) {
	if result, err := b.GRPCGetTransactionByHash(txHash); err == nil {
		return result, nil
	}
	var result *GetTxResponse
	var err error
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, TxByHash+txHash)
		if err = client.RPCGet(&result, restApi); err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetTransactionByHash")
}

func (b *Bridge) GetBaseAccount(address string) (*QueryAccountResponse, error) {
	if result, err := b.GRPCGetBaseAccount(address); err == nil {
		return result, nil
	}
	var result *QueryAccountResponse
	var err error
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, AccountInfo+address)
		if err = client.RPCGet(&result, restApi); err == nil {
			return result, nil
		} else {
			log.Warn("GetBaseAccount failed", "url", restApi, "err", err)
		}
	}
	return nil, wrapRPCQueryError(err, "GetBaseAccount")
}

func (b *Bridge) GetDenomBalance(address, denom string) (sdk.Int, error) {
	if result, err := b.GRPCGetDenomBalance(address, denom); err == nil {
		return result, nil
	}
	var result *QueryAllBalancesResponse
	var err error
	for _, url := range b.AllGatewayURLs {
		restApi := joinURLPath(url, Balances+address)
		if err = client.RPCGet(&result, restApi); err == nil {
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
	return sdk.ZeroInt(), wrapRPCQueryError(err, "GetDenomBalance")
}

func (b *Bridge) SimulateTx(simulateReq *SimulateRequest) (string, error) {
	if result, err := b.GRPCSimulateTx(simulateReq); err == nil {
		return common.ToJSONString(result.GasInfo, false), nil
	}
	if data, err := json.Marshal(simulateReq); err != nil {
		return "", err
	} else {
		for _, url := range b.AllGatewayURLs {
			restApi := joinURLPath(url, SimulateTx)
			if res, err := client.RPCRawPostWithTimeout(restApi, string(data), 120); err == nil && res != "" && res != "\n" {
				return res, nil
			}
		}
		return "", wrapRPCQueryError(err, "SimulateTx")
	}
}

func (b *Bridge) BroadcastTx(req *BroadcastTxRequest) (string, error) {
	if result, err := b.GRPCBroadcastTx(req); err == nil {
		return result.TxHash, nil
	}
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
		return "", wrapRPCQueryError(err, "BroadcastTx")
	}
}

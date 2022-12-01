package reef

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

var (
	errEmptyURLs = errors.New("empty URLs")
	// errTxInOrphanBlock        = errors.New("tx is in orphan block")
	errTxHashMismatch         = errors.New("tx hash mismatch with rpc result")
	errTxBlockHashMismatch    = errors.New("tx block hash mismatch with rpc result")
	errTxReceiptMissBlockInfo = errors.New("tx receipt missing block info")

	wrapRPCQueryError = tokens.WrapRPCQueryError
)

func (b *Bridge) GetLatestBlockNumberOf(url string) (latest uint64, err error) {
	var result map[string]interface{}
	// chain_getFinalizedHead ?
	err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "chain_getHeader")
	if err == nil {
		return common.GetUint64FromStr(result["number"].(string))
	}
	return 0, wrapRPCQueryError(err, "chain_getHeader")
}

func (b *Bridge) GetLatestBlockNumber() (maxHeight uint64, err error) {
	gateway := b.GatewayConfig
	var height uint64
	for _, url := range gateway.APIAddress {
		height, err = b.GetLatestBlockNumberOf(url)
		if height > maxHeight && err == nil {
			maxHeight = height
		}
	}
	if maxHeight > 0 {
		return maxHeight, nil
	}
	return 0, wrapRPCQueryError(err, "chain_getHeader")
}

func (b *Bridge) GetGetBlockHash(blockNumber uint64) (blockHash string, err error) {
	gateway := b.GatewayConfig
	var result map[string]interface{}
	param := strconv.FormatUint(blockNumber, 16)
	for _, url := range gateway.APIAddress {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "chain_getHeader", param)
		if err == nil {
			return result["result"].(string), nil
		}
	}
	return "", wrapRPCQueryError(err, "chain_getHeader")
}

// CallContract call eth_call
func (b *Bridge) CallContract(contract string, data hexutil.Bytes, blockNumber string) (string, error) {
	reqArgs := map[string]interface{}{
		"to":           contract,
		"data":         data,
		"storageLimit": 0,
	}
	var err error
LOOP:
	for _, url := range b.AllGatewayURLs {
		var result string
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_call", reqArgs)
		if err != nil && router.IsIniting {
			for i := 0; i < router.RetryRPCCountInInit; i++ {
				if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_call", reqArgs); err == nil {
					return result, nil
				}
				if strings.Contains(err.Error(), "execution reverted") {
					break LOOP
				}
				time.Sleep(router.RetryRPCIntervalInInit)
			}
		}
		if err == nil {
			return result, nil
		}
	}
	if err != nil {
		logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
		logFunc("call CallContract failed", "contract", contract, "data", data, "err", err)
	}
	return "", wrapRPCQueryError(err, "eth_call", contract)
}

func (b *Bridge) QueryEvmAddress(ss58address string) (addr *common.Address, err error) {
	for _, ws := range b.WS {
		addr, err = ws.QueryEvmAddress(ss58address)
		if err != nil {
			log.Warn("QueryEvmAddress", "err", err)
		}
		if addr != nil {
			return addr, nil
		}
	}
	return addr, fmt.Errorf("reef QueryEvmAddress address %s not register evm address ", ss58address)
}

// GetBalance call eth_getBalance
func (b *Bridge) GetBalance(account string) (balance *big.Int, err error) {
	key, err := types.CreateStorageKey(b.MetaData, "System", "Account", AddressToPubkey(account))
	if err != nil {
		return
	}
	var accountInfo types.AccountInfo
	for _, api := range b.SubstrateAPIs {
		ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
		if err != nil || !ok {
			log.Warn("reef getBalance", "account", account, "err", err)
			continue
		}
		balance = new(big.Int).SetUint64(accountInfo.Data.Free.Uint64())
		break
	}
	return
}

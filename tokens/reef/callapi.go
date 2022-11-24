package reef

import (
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
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
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_call", reqArgs, blockNumber)
		if err != nil && router.IsIniting {
			for i := 0; i < router.RetryRPCCountInInit; i++ {
				if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_call", reqArgs, blockNumber); err == nil {
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

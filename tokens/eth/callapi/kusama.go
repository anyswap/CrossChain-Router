package callapi

import (
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	wrapRPCQueryError = tokens.WrapRPCQueryError
)

// ------------------------ kusama specific apis -----------------------------

// KsmGetLatestBlockNumberOf get latest block number
func KsmGetLatestBlockNumberOf(url string, gateway *tokens.GatewayConfig, timeout int) (latest uint64, err error) {
	blockHash, err := KsmGetFinalizedHead(url, timeout)
	if err != nil {
		return 0, err
	}
	header, err := KsmGetHeader(blockHash.String(), gateway, timeout)
	if err != nil {
		return 0, err
	}
	return header.Number.ToInt().Uint64(), nil
}

// KsmGetFinalizedHead call chain_getFinalizedHead
func KsmGetFinalizedHead(url string, timeout int) (result common.Hash, err error) {
	err = client.RPCPostWithTimeout(timeout, &result, url, "chain_getFinalizedHead")
	if err == nil {
		return result, nil
	}
	return result, wrapRPCQueryError(err, "chain_getFinalizedHead")
}

// KsmHeader struct
type KsmHeader struct {
	ParentHash *common.Hash `json:"parentHash"`
	Number     *hexutil.Big `json:"number"`
}

// KsmGetHeader call chain_getHeader
func KsmGetHeader(blockHash string, gateway *tokens.GatewayConfig, timeout int) (result *KsmHeader, err error) {
	result, err = ksmGetHeader(blockHash, gateway.APIAddress, timeout)
	if err != nil && len(gateway.APIAddressExt) > 0 {
		result, err = ksmGetHeader(blockHash, gateway.APIAddressExt, timeout)
	}
	return result, err
}

func ksmGetHeader(blockHash string, urls []string, timeout int) (result *KsmHeader, err error) {
	for _, url := range urls {
		err = client.RPCPostWithTimeout(timeout, &result, url, "chain_getHeader", blockHash)
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "chain_getHeader", blockHash)
}

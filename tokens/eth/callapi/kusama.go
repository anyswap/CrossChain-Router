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

// ------------------------ kusama override apis -----------------------------

// KsmHeader struct
type KsmHeader struct {
	ParentHash *common.Hash `json:"parentHash"`
	Number     *hexutil.Big `json:"number"`
}

// KsmGetFinalizedBlockNumber call chain_getFinalizedHead and chain_getHeader
func KsmGetFinalizedBlockNumber(b tokens.IBridge) (latest uint64, err error) {
	gateway := b.GetGatewayConfig()
	urls := append(gateway.APIAddress, gateway.APIAddressExt...)
	blockHash, err := KsmGetFinalizedHead(urls)
	if err != nil {
		return 0, err
	}
	header, err := KsmGetHeader(urls, blockHash.String())
	if err != nil {
		return 0, err
	}
	return header.Number.ToInt().Uint64(), nil
}

// ------------------------ kusama specific apis -----------------------------

// KsmGetFinalizedHead call chain_getFinalizedHead
func KsmGetFinalizedHead(urls []string) (result *common.Hash, err error) {
	for _, url := range urls {
		err = client.RPCPost(&result, url, "chain_getFinalizedHead")
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "chain_getFinalizedHead")
}

// KsmGetHeader call chain_getHeader
func KsmGetHeader(urls []string, blockHash string) (result *KsmHeader, err error) {
	for _, url := range urls {
		err = client.RPCPost(&result, url, "chain_getHeader", blockHash)
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "chain_getHeader", blockHash)
}

package callapi

import (
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// ------------------------ conflux override apis -----------------------------

// CfxBlock struct
type CfxBlock struct {
	Hash        *common.Hash `json:"hash"`
	ParentHash  *common.Hash `json:"parentHash"`
	EpochNumber *hexutil.Big `json:"epochNumber"`
	BlockNumber *hexutil.Big `json:"blockNumber"`
}

// CfxGetFinalizedBlockNumber call cfx_getBlockByEpochNumber
func CfxGetFinalizedBlockNumber(b tokens.IBridge) (latest uint64, err error) {
	urls := b.GetGatewayConfig().FinalizeAPIAddress
	var maxHeight uint64
	for _, url := range urls {
		var result *CfxBlock
		err = client.RPCPost(&result, url, "cfx_getBlockByEpochNumber", "latest_finalized", false)
		if err == nil && result != nil {
			h := result.EpochNumber.ToInt().Uint64()
			if h > maxHeight {
				maxHeight = h
			}
		}
	}
	if maxHeight > 0 {
		return maxHeight, nil
	}
	return 0, wrapRPCQueryError(err, "cfx_getBlockByEpochNumber")
}

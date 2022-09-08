package cosmos

import (
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/cosmos/cosmos-sdk/api/tendermint/types"
)

const (
	LatestBlock = "/cosmos/base/tendermint/v1beta1/blocks/latest"
)

func (c *CosmosRestClient) GetLatestBlockNumber() (uint64, error) {
	var result types.Block
	for _, url := range c.BaseUrls {
		restApi := url + LatestBlock
		if err := client.RPCGet(&result, restApi); err == nil {
			return uint64(result.Header.Height), nil
		}
	}
	return 0, tokens.ErrNotImplemented
}

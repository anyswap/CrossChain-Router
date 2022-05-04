package bridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tron"
)

// NewCrossChainBridge new bridge
func NewCrossChainBridge(chainID *big.Int) tokens.IBridge {
	switch chainID.Uint64() {
	case tron.TronMainnetChainID, tron.TronShastaChainID :
		return tron.NewCrossChainBridge()
	default:
		return eth.NewCrossChainBridge()
	}
}

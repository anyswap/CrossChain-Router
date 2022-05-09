package bridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
	"github.com/anyswap/CrossChain-Router/v3/tokens/reef"
)

// NewCrossChainBridge new bridge
func NewCrossChainBridge(chainID *big.Int) tokens.IBridge {
	switch {
	case chainID.Sign() <= 0:
		log.Fatal("wrong chainID", "chainID", chainID)
	case chainID.Uint64() == reef.ChainID:
		return reef.NewCrossChainBridge()
	default:
		return eth.NewCrossChainBridge()
	}
	return nil
}

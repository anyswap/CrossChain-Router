package bridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
	"github.com/anyswap/CrossChain-Router/v3/tokens/btc"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
	"github.com/anyswap/CrossChain-Router/v3/tokens/flow"
	"github.com/anyswap/CrossChain-Router/v3/tokens/iota"
	"github.com/anyswap/CrossChain-Router/v3/tokens/near"
	"github.com/anyswap/CrossChain-Router/v3/tokens/reef"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
	"github.com/anyswap/CrossChain-Router/v3/tokens/stellar"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tron"
)

// NewCrossChainBridge new bridge
func NewCrossChainBridge(chainID *big.Int) tokens.IBridge {
	plog := func(chainID *big.Int, chain string) {
		log.Info("init new cross chain", "chain", chain, "chainID", chainID)
	}

	switch {
	case reef.SupportsChainID(chainID):
		plog(chainID, "reef")
		return reef.NewCrossChainBridge()
	case solana.SupportChainID(chainID):
		plog(chainID, "solana")
		return solana.NewCrossChainBridge()
	case cosmos.SupportsChainID(chainID):
		plog(chainID, "cosmos")
		return cosmos.NewCrossChainBridge()
	case btc.SupportsChainID(chainID):
		plog(chainID, "btc")
		return btc.NewCrossChainBridge()
	case cardano.SupportsChainID(chainID):
		plog(chainID, "cardano")
		return cardano.NewCrossChainBridge()
	case aptos.SupportsChainID(chainID):
		plog(chainID, "aptos")
		return aptos.NewCrossChainBridge()
	case tron.SupportsChainID(chainID):
		plog(chainID, "tron")
		return tron.NewCrossChainBridge()
	case near.SupportsChainID(chainID):
		plog(chainID, "near")
		return near.NewCrossChainBridge()
	case iota.SupportsChainID(chainID):
		plog(chainID, "iota")
		return iota.NewCrossChainBridge()
	case ripple.SupportsChainID(chainID):
		plog(chainID, "ripple")
		return ripple.NewCrossChainBridge()
	case stellar.SupportsChainID(chainID):
		plog(chainID, "stellar")
		return stellar.NewCrossChainBridge(chainID.String())
	case flow.SupportsChainID(chainID):
		plog(chainID, "flow")
		return flow.NewCrossChainBridge()
	case chainID.Sign() <= 0:
		log.Fatal("wrong chainID", "chainID", chainID)
	default:
		plog(chainID, "ethlike")
		return eth.NewCrossChainBridge()
	}
	return nil
}

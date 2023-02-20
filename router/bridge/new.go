package bridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
	"github.com/anyswap/CrossChain-Router/v3/tokens/btc"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
	"github.com/anyswap/CrossChain-Router/v3/tokens/near"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tron"
	"github.com/anyswap/CrossChain-Router/v3/tokens/wrapper"
)

// NewCrossChainBridge new bridge
func NewCrossChainBridge(chainID *big.Int) tokens.IBridge {
	cfg := params.GetRouterConfig()
	wrapperCfg := cfg.WrapperGateways[chainID.String()]
	switch {
	case wrapperCfg != nil && wrapperCfg.RPCAddress != "":
		return wrapper.NewCrossChainBridge(wrapperCfg)
	case solana.SupportChainID(chainID):
		return solana.NewCrossChainBridge()
	case cosmos.SupportsChainID(chainID):
		return cosmos.NewCrossChainBridge()
	case btc.SupportsChainID(chainID):
		return btc.NewCrossChainBridge()
	case cardano.SupportsChainID(chainID):
		return cardano.NewCrossChainBridge()
	case aptos.SupportsChainID(chainID):
		return aptos.NewCrossChainBridge()
	case tron.SupportsChainID(chainID):
		return tron.NewCrossChainBridge()
	case near.SupportsChainID(chainID):
		return near.NewCrossChainBridge()
	case ripple.SupportsChainID(chainID):
		return ripple.NewCrossChainBridge()
	case chainID.Sign() <= 0:
		log.Fatal("wrong chainID", "chainID", chainID)
	default:
		return eth.NewCrossChainBridge()
	}
	return nil
}

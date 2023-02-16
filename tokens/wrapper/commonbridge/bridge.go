package commonbridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/wrapper/impl"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
)

// Bridge bridge
type Bridge struct {
	*tokens.CrossChainBridgeBase
	*impl.Bridge
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge(cfg *impl.BridgeConfig) *Bridge {
	return &Bridge{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
		Bridge:               impl.NewCrossChainBridge(cfg),
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	b.Bridge.InitAfterConfig()
}

// GetBalance get balance is used for checking budgets to prevent DOS attacking
func (b *Bridge) GetBalance(account string) (*big.Int, error) {
	return b.Bridge.GetBalance(account)
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
	return b.Bridge.InitRouterInfo(routerContract, routerVersion)
}

package noncebridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/anyswap/CrossChain-Router/v3/tokens/wrapper/impl"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
)

// Bridge bridge
type Bridge struct {
	*base.NonceSetterBase
	*impl.Bridge
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge(cfg *params.WrapperConfig) *Bridge {
	return &Bridge{
		NonceSetterBase: base.NewNonceSetterBase(),
		Bridge:          impl.NewCrossChainBridge(cfg),
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

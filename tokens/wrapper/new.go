// Package wrapper implement `tokens.IBridge` by access external components.
// communicate with components through designed rpc call interfaces.
package wrapper

import (
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/wrapper/commonbridge"
	"github.com/anyswap/CrossChain-Router/v3/tokens/wrapper/noncebridge"
)

// NewCrossChainBridge new bridge
func NewCrossChainBridge(cfg *params.WrapperConfig) tokens.IBridge {
	if cfg.SupportNonce {
		return noncebridge.NewCrossChainBridge(cfg)
	}
	return commonbridge.NewCrossChainBridge(cfg)
}

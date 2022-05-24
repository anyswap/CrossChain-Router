package reef

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
)

const (
	ChainID = uint64(13939) // mainnet and testnet are same
)

// Bridge eth bridge
type Bridge struct {
	*eth.Bridge
	WatcherAccount string
}

func (b *Bridge) IsSubstrate() bool {
	return false
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		eth.NewCrossChainBridge(),
		"0x548cA69C510E0E2d5ae562B633fCEe5480Bed375",
	}
}
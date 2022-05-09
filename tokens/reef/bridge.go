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
}
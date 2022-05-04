package tron

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// GetPairFor call "getPair(address,address)"
func (b *Bridge) GetPairFor(factory, token0, token1 string) (string, error) {
	return tokens.GetPairFor(factory, token0, token1)
}

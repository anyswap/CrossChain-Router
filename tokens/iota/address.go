package iota

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	return true
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return "", tokens.ErrNotImplemented
}

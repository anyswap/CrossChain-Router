package eth

import (
	"strings"
)

// NonceSetterBase base nonce setter
type NonceSetterBase struct {
	SwapNonce map[string]uint64 // key is sender address
}

// NewNonceSetterBase new base nonce setter
func NewNonceSetterBase() *NonceSetterBase {
	return &NonceSetterBase{
		SwapNonce: make(map[string]uint64),
	}
}

// AdjustNonce adjust account nonce (eth like chain)
func (b *Bridge) AdjustNonce(address string, value uint64) (nonce uint64) {
	account := strings.ToLower(address)
	if b.SwapNonce[account] > value {
		nonce = b.SwapNonce[account]
	} else {
		b.SwapNonce[account] = value
		nonce = value
	}
	return nonce
}

// IncreaseNonce decrease account nonce (eth like chain)
func (b *Bridge) IncreaseNonce(address string, value uint64) {
	account := strings.ToLower(address)
	b.SwapNonce[account] += value
}

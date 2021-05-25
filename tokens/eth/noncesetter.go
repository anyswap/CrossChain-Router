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
		nonce = value
	}
	return nonce
}

// SetNonce set account nonce (eth like chain)
func (b *Bridge) SetNonce(address string, value uint64) {
	account := strings.ToLower(address)
	if b.SwapNonce[account] < value {
		b.SwapNonce[account] = value
	}
}

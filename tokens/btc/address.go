package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	LenForPubKey  = 128
	AddressLength = 16
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return true
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	return "", tokens.ErrNotImplemented
}

// todo： read from config
func (b *Bridge) GetAccountNonce(address, pubKey string) (uint64, error) {
	return 0, tokens.ErrNotImplemented
}

// todo： read from config
func (b *Bridge) VerifyPubKey(address, pubKey string) error {
	return tokens.ErrNotImplemented
}

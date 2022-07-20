package ripple

import (
	"regexp"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var rAddressReg = regexp.MustCompile(`^r[1-9a-km-zA-HJ-NP-Z]{32,33}(?::[0-9]*)?$`)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	return true
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return "", tokens.ErrNotImplemented
}

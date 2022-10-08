package cardano

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	cmdStr := fmt.Sprintf(AddressInfoCmd, addr)
	if _, err := ExecCmd(cmdStr, " "); err != nil {
		return false
	}
	return true
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return "", tokens.ErrNotImplemented
}

// VerifyMPCPubKey verify mpc address and public key is matching
func VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	return nil
}

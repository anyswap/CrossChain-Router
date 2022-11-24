package reef

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

const mpc_publickey_type = "sr25519"

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	if !common.IsHexAddress(address) {
		return false
	}
	unprefixedHex, ok, hasUpperChar := common.GetUnprefixedHex(address)
	if hasUpperChar {
		if strings.ToUpper(address) == address {
			return true
		}
		// valid checksum
		if unprefixedHex != common.HexToAddress(address).Hex()[2:] {
			return false
		}
	}
	return ok
}

func PublicKeyToAddress(pubKey string) (string, error) {
	return Public2address(mpc_publickey_type, pubKey)
}

// VerifyMPCPubKey verify mpc address and public key is matching
func VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	if !common.IsHexAddress(mpcAddress) {
		return fmt.Errorf("wrong mpc address '%v'", mpcAddress)
	}
	pubkeyAddr, err := PublicKeyToAddress(mpcPubkey)
	if err != nil {
		return err
	}
	if !strings.EqualFold(pubkeyAddr, mpcAddress) {
		return fmt.Errorf("mpc address %v and public key address %v is not match", mpcAddress, pubkeyAddr)
	}
	return nil
}

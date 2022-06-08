package ripple

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

var rAddressReg = regexp.MustCompile(`^r[1-9a-km-zA-HJ-NP-Z]{32,33}(?::[0-9]*)?$`)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	return rAddressReg.MatchString(addr)
}

// GetAddressAndTag get address and tag
func GetAddressAndTag(s string) (addr string, tag *uint32, err error) {
	if !rAddressReg.MatchString(s) {
		return "", nil, fmt.Errorf("invalid address '%s'", s)
	}
	parts := strings.Split(s, ":")
	addr = parts[0]
	if len(parts) == 2 {
		var tagVal uint32
		tagVal, err = common.GetUint32FromStr(parts[1])
		if err != nil {
			return "", nil, err
		}
		tag = &tagVal
		return addr, tag, nil
	}
	return addr, nil, nil
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return PublicKeyHexToAddress(pubKeyHex)
}

// PublicKeyHexToAddress convert public key hex to ripple address
func PublicKeyHexToAddress(pubKeyHex string) (string, error) {
	pub, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return "", err
	}
	return PublicKeyToAddress(pub), nil
}

// PublicKeyToAddress converts pubkey to ripple address
func PublicKeyToAddress(pubkey []byte) string {
	return GetAddress(ImportPublicKey(pubkey), nil)
}

// VerifyMPCPubKey verify mpc address and public key is matching
func VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	pubkeyAddr, err := PublicKeyHexToAddress(mpcPubkey)
	if err != nil {
		return err
	}
	if !strings.EqualFold(pubkeyAddr, mpcAddress) {
		return fmt.Errorf("mpc address %v and public key address %v is not match", mpcAddress, pubkeyAddr)
	}
	return nil
}

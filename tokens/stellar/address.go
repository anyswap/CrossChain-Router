package stellar

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/stellar/go/strkey"
)

var rAddressReg = "^[1-9A-Z]{56}$"

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	match, err := regexp.MatchString(rAddressReg, addr)
	if err != nil {
		log.Warn("Error occurs when verify address", "error", err)
	}
	return match
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	return PublicKeyHexToAddress(pubKey)
}

// PublicKeyHexToAddress convert public key hex to ripple address
func PublicKeyHexToAddress(pubKeyHex string) (string, error) {
	pub, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return "", err
	}
	return PublicKeyToAddress(pub)
}

func PublicKeyToAddress(pubkey []byte) (string, error) {
	pubkeyAddr, err := strkey.Encode(strkey.VersionByteAccountID, pubkey)
	if err != nil {
		return "", err
	}
	return pubkeyAddr, nil
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

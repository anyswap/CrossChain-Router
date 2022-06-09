package stellar

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/stellar/go/strkey"
)

var rAddressReg = "^[1-9A-Z]{56}$"

// 严格验证
// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	match, err := regexp.MatchString(rAddressReg, addr)
	if err != nil {
		log.Warn("Error occurs when verify address", "error", err)
		return match
	}
	_, err = strkey.Decode(strkey.VersionByteAccountID, addr)
	return err == nil
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	return PublicKeyHexToAddress(pubKey)
}

// PublicKeyHexToAddress convert public key hex to stellar address
func PublicKeyHexToAddress(pubKeyHex string) (string, error) {
	pubKey := pubKeyHex
	// 去掉0xED
	if pubKeyHex[:2] == "0x" || pubKeyHex[:2] == "0X" {
		pubKey = pubKey[2:]
	}
	pub, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", err
	}
	if len(pub) == ed25519.PublicKeySize+1 && pub[0] == 0xED {
		return PublicKeyToAddress(pub[1:])
	} else if len(pub) == ed25519.PublicKeySize {
		return PublicKeyToAddress(pub)
	} else {
		return "", fmt.Errorf("public key format error : %v", pubKeyHex)
	}
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

func FormatPublicKeyToPureHex(pubKeyHex string) (string, error) {
	pubKey := pubKeyHex
	// 去掉0xED
	if pubKeyHex[:2] == "0x" || pubKeyHex[:2] == "0X" {
		pubKey = pubKey[2:]
	}
	pub, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", err
	}
	if len(pub) == ed25519.PublicKeySize+1 && pub[0] == 0xED {
		return pubKeyHex[4:], nil
	} else if len(pub) == ed25519.PublicKeySize {
		return pubKeyHex, nil
	} else {
		return "", fmt.Errorf("public key format error : %v", pubKeyHex)
	}
}

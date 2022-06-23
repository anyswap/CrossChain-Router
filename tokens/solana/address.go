package solana

import (
	"crypto/ed25519"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

// IsValidAddress impl check address
func (b *Bridge) IsValidAddress(address string) bool {
	if b.IsNative(address) {
		return true
	}
	_, err := types.PublicKeyFromBase58(address)
	return err == nil
}

func (b *Bridge) IsNative(address string) bool {
	return strings.ToLower(address) == "native"
}

// VerifyMPCPubKey verify mpc address and public key is matching
func (b *Bridge) VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	pubAddr, err := b.PublicKeyToAddress(mpcPubkey)
	log.Info("VerifyMPCPubKey : ", pubAddr, mpcAddress)
	if err != nil || pubAddr != mpcAddress {
		return fmt.Errorf("mpc address %v and public key address %v is not match", mpcAddress, pubAddr)
	}
	return nil
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	pubKey := pubKeyHex
	if common.HasHexPrefix(pubKey) {
		pubKey = pubKey[2:]
	}
	pub := common.FromHex(pubKey)
	if len(pub) == ed25519.PublicKeySize+1 && pub[0] == 0xED {
		return types.PublicKeyFromBytes((pub[1:])).String(), nil
	}
	if len(pub) == ed25519.PublicKeySize {
		return types.PublicKeyFromBytes(pub).String(), nil
	}
	return "", fmt.Errorf("pubKeyHex format error : %v", pubKeyHex)
}

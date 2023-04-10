package near

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return address != ""
}

func (b *Bridge) GetAccountNonce(account, publicKey string) (uint64, error) {
	urls := b.GatewayConfig.AllGatewayURLs
	for _, url := range urls {
		result, err := GetAccountNonce(url, account, publicKey)
		if err == nil {
			return result, nil
		}
	}
	return 0, tokens.ErrGetAccountNonce
}

// PublicKeyToAddress public key hex string (may be uncompressed) to address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	nearPubKey, err := PublicKeyFromHexString(pubKeyHex)
	if err != nil {
		return "", err
	}
	return nearPubKey.Address(), nil
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	nearPubKey, err := PublicKeyFromString(pubkey)
	if err != nil {
		return err
	}
	if common.IsHexHash(address) {
		if !strings.EqualFold(nearPubKey.Address(), address) {
			return fmt.Errorf("address %v and public key %v is not match", address, pubkey)
		}
	}
	_, err = b.GetAccountNonce(address, pubkey)
	if err != nil {
		return fmt.Errorf("verify public key failed, %w", err)
	}
	return nil
}

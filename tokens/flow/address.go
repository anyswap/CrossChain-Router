package flow

import (
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	LenForPubKey = 128
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return address != ""
}

func (b *Bridge) pubKeyToMpcPubKey(pubKey string) (string, error) {
	if len(pubKey) != LenForPubKey {
		return "", errors.New("pubKey len not match")
	}
	return fmt.Sprintf("04%s", pubKey), nil
}

func (b *Bridge) pubKeyToAccountKey(pubKey string) (string, error) {
	if len(pubKey) != LenForPubKey {
		return "", errors.New("pubKey len not match")
	}
	return fmt.Sprintf("0x%s", pubKey), nil
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	return "", tokens.ErrNotImplemented
}

func (b *Bridge) GetAccountNonce(address string) (uint64, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetAccount(url, address)
		if err == nil {
			return result.Keys[0].SequenceNumber, nil
		}
	}
	return 0, tokens.ErrGetAccount
}

func (b *Bridge) GetAccountIndex(address string) (int, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetAccount(url, address)
		if err == nil {
			return result.Keys[0].Index, nil
		}
	}
	return 0, tokens.ErrGetAccount
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetAccount(url, address)
		if err == nil {
			for _, key := range result.Keys {
				realPubKey, err := b.pubKeyToAccountKey(pubkey)
				if err != nil {
					continue
				}
				if key.PublicKey.String() == realPubKey {
					return nil
				}
			}
		}
	}
	return errors.New("not such pubKey for this address")
}

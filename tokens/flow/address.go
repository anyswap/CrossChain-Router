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

func (b *Bridge) PubKeyToMpcPubKey(pubKey string) (string, error) {
	if len(pubKey) != LenForPubKey {
		return "", errors.New("pubKey len not match")
	}
	return fmt.Sprintf("04%s", pubKey), nil
}

func (b *Bridge) PubKeyToAccountKey(pubKey string) (string, error) {
	if len(pubKey) != LenForPubKey {
		return "", errors.New("pubKey len not match")
	}
	return fmt.Sprintf("0x%s", pubKey), nil
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	return "", tokens.ErrNotImplemented
}

func (b *Bridge) GetAccountNonce(address, pubKey string) (uint64, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetAccount(url, address)
		if err == nil {
			for _, key := range result.Keys {
				realPubKey, err := b.PubKeyToAccountKey(pubKey)
				if err != nil {
					continue
				}
				if key.PublicKey.String() == realPubKey {
					return key.SequenceNumber, nil
				}
			}
		}
	}
	return 0, tokens.ErrGetAccount
}

func (b *Bridge) GetAccountIndex(address, pubKey string) (int, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetAccount(url, address)
		if err == nil {
			for _, key := range result.Keys {
				realPubKey, err := b.PubKeyToAccountKey(pubKey)
				if err != nil {
					continue
				}
				if key.PublicKey.String() == realPubKey {
					return key.Index, nil
				}
			}
		}
	}
	return 0, tokens.ErrGetAccount
}

func (b *Bridge) VerifyPubKey(address, pubKey string) error {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetAccount(url, address)
		if err == nil {
			for _, key := range result.Keys {
				realPubKey, err := b.PubKeyToAccountKey(pubKey)
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

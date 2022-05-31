package flow

import "github.com/anyswap/CrossChain-Router/v3/tokens"

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return address != ""
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

// PublicKeyToAddress public key hex string (may be uncompressed) to address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return "", nil
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	return nil
}

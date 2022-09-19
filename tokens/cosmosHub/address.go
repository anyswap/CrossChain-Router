package cosmosHub

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return cosmosSDK.IsValidAddress(address)
}

// PublicKeyToAddress public key hex string (may be uncompressed) to address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return cosmosSDK.PublicKeyToAddress(pubKeyHex)
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	return cosmosSDK.VerifyPubKey(address, pubkey)
}

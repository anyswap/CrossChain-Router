package cosmosRouter

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	prefix := b.GetChainConfig().Extra
	return cosmosSDK.IsValidAddress(prefix, address)
}

// PublicKeyToAddress public key hex string (may be uncompressed) to address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	prefix := b.GetChainConfig().Extra
	return cosmosSDK.PublicKeyToAddress(prefix, pubKeyHex)
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	prefix := b.GetChainConfig().Extra
	return cosmosSDK.VerifyPubKey(address, prefix, pubkey)
}

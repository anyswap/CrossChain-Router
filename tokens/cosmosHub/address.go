package cosmosHub

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return b.CosmosRestClient.IsValidAddress(address)
}

// PublicKeyToAddress public key hex string (may be uncompressed) to address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return b.CosmosRestClient.PublicKeyToAddress(pubKeyHex)
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	return tokens.ErrNotImplemented
}

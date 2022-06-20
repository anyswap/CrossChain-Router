package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return true
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	pkData := common.FromHex(pubKey)
	cPkData, err := b.ToCompressedPublicKey(pkData)
	if err != nil {
		return "", err
	}
	address, err := b.NewAddressPubKeyHash(cPkData)
	if err != nil {
		return "", err
	}
	return address.EncodeAddress(), nil
}

// todo： read from config
func (b *Bridge) VerifyPubKey(address, pubKey string) error {
	return tokens.ErrNotImplemented
}

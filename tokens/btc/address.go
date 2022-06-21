package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/common"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	_, err := b.DecodeAddress(address)
	return err == nil
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
	wantAddr, err := b.PublicKeyToAddress(pubKey)
	if err != nil || wantAddr != address {
		return err
	}
	return nil
}

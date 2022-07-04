package block

import (
	"fmt"

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

// VerifyPubKey verify address and public key is matched
func (b *Bridge) VerifyPubKey(address, pubKey string) error {
	wantAddr, err := b.PublicKeyToAddress(pubKey)
	if err != nil {
		return err
	}
	if wantAddr != address {
		return fmt.Errorf("address %v and public key %v mismatch", address, pubKey)
	}
	return nil
}

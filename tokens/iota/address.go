package iota

import (
	"encoding/hex"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/common"
	iotago "github.com/iotaledger/iota.go/v2"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	prefix, _, err := iotago.ParseBech32(addr)
	return err == nil && string(prefix) == b.Prefix
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(edPubKey string) (string, error) {
	return PublicKeyToAddress(b.Prefix, edPubKey)
}

func PublicKeyToAddress(prefix, edPubKey string) (string, error) {
	edAddr := ConvertStringToAddress(edPubKey)
	bech32Addr := edAddr.Bech32(iotago.NetworkPrefix(prefix))
	return bech32Addr, nil
}

func HexPublicKeyToAddress(prefix, pubKeyHex string) (string, error) {
	if common.HasHexPrefix(pubKeyHex) {
		pubKeyHex = pubKeyHex[2:]
	}
	publicKey, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return "", err
	}
	edAddr := iotago.AddressFromEd25519PubKey(publicKey)
	bech32Addr := edAddr.Bech32(iotago.NetworkPrefix(prefix))
	return bech32Addr, nil
}

// VerifyMPCPubKey verify mpc address and public key is matching
func VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	edAddr := ConvertPubKeyToAddr(mpcPubkey)
	if edAddr.String() == mpcAddress {
		return nil
	}
	return errors.New("VerifyMPCPubKey eror")
}

func Bech32ToEdAddr(bech32 string) (*iotago.Address, error) {
	if _, edAddr, err := iotago.ParseBech32(bech32); err == nil {
		return &edAddr, nil
	} else {
		return nil, err
	}
}

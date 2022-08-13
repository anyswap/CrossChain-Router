package cosmos

import (
	"encoding/hex"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/btcsuite/btcd/btcec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func IsValidAddress(address string) bool {
	if _, err := sdk.AccAddressFromBech32(address); err != nil {
		return false
	}
	return true
}

func PublicKeyToAddress(pubKeyHex string) (string, error) {
	if pk, err := PubKeyFromStr(pubKeyHex); err != nil {
		return "", err
	} else {
		if accAddress, err := sdk.AccAddressFromHexUnsafe(pk.Address().String()); err != nil {
			return "", err
		} else {
			return accAddress.String(), nil
		}
	}
}

// PubKeyFromStr get public key from hex string
func PubKeyFromStr(pubKeyHex string) (cryptotypes.PubKey, error) {
	pubKeyHex = strings.TrimPrefix(pubKeyHex, "0x")
	if bs, err := hex.DecodeString(pubKeyHex); err != nil {
		return nil, err
	} else {
		return PubKeyFromBytes(bs)
	}
}

// PubKeyFromBytes get public key from bytes
func PubKeyFromBytes(pubKeyBytes []byte) (cryptotypes.PubKey, error) {
	if cmp, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256()); err != nil {
		return nil, err
	} else {
		compressedPublicKey := make([]byte, secp256k1.PubKeySize)
		copy(compressedPublicKey, cmp.SerializeCompressed())

		return &secp256k1.PubKey{Key: compressedPublicKey}, nil
	}
}

func VerifyPubKey(address, pubkey string) error {
	if addr, err := PublicKeyToAddress(pubkey); err != nil {
		return err
	} else {
		if address != addr {
			return tokens.ErrValidPublicKey
		}
		return nil
	}
}

func (c *CosmosRestClient) VerifyPubKey(address, pubkey string) error {
	if addr, err := c.PublicKeyToAddress(pubkey); err != nil {
		return err
	} else {
		if address != addr {
			return tokens.ErrValidPublicKey
		}
		return nil
	}
}

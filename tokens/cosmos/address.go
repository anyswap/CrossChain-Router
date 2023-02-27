package cosmos

import (
	"encoding/hex"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/btcsuite/btcd/btcec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	return IsValidAddress(b.Prefix, address)
}

// PublicKeyToAddress public key hex string (may be uncompressed) to address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	return PublicKeyToAddress(b.Prefix, pubKeyHex)
}

func (b *Bridge) VerifyPubKey(address, pubkey string) error {
	return VerifyPubKey(address, b.Prefix, pubkey)
}

func IsValidAddress(prefix, address string) bool {
	if bz, err := sdk.GetFromBech32(address, prefix); err == nil {
		if err = sdk.VerifyAddressFormat(bz); err == nil {
			accAddress := sdk.AccAddress(bz)
			if bech32Addr, err := bech32.ConvertAndEncode(prefix, accAddress); err == nil && bech32Addr == address {
				return true
			}
		} else {
			log.Warnf("invalid address format %v: %v", address, err)
		}
	} else {
		log.Warnf("invalid address %v: %v", address, err)
	}
	return false
}

func PublicKeyToAddress(prefix, pubKeyHex string) (string, error) {
	if pk, err := PubKeyFromStr(pubKeyHex); err != nil {
		return "", err
	} else {
		if accAddress, err := sdk.AccAddressFromHex(pk.Address().String()); err != nil {
			return "", err
		} else {
			if bech32Addr, err := bech32.ConvertAndEncode(prefix, accAddress); err == nil {
				return bech32Addr, nil
			} else {
				return "", err
			}
		}
	}
}

// PubKeyFromStr get public key from hex string
func PubKeyFromStr(pubKeyHex string) (cryptoTypes.PubKey, error) {
	pubKeyHex = strings.TrimPrefix(pubKeyHex, "0x")
	if bs, err := hex.DecodeString(pubKeyHex); err != nil {
		return nil, err
	} else {
		return PubKeyFromBytes(bs)
	}
}

// PubKeyFromBytes get public key from bytes
func PubKeyFromBytes(pubKeyBytes []byte) (cryptoTypes.PubKey, error) {
	if cmp, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256()); err != nil {
		return nil, err
	} else {
		compressedPublicKey := make([]byte, secp256k1.PubKeySize)
		copy(compressedPublicKey, cmp.SerializeCompressed())

		return &secp256k1.PubKey{Key: compressedPublicKey}, nil
	}
}

func VerifyPubKey(address, prefix, pubkey string) error {
	if addr, err := PublicKeyToAddress(prefix, pubkey); err != nil {
		log.Warn("public key to address error", "pubkey", pubkey, "prefix", prefix, "err", err)
		return err
	} else {
		if address != addr {
			return tokens.ErrValidPublicKey
		}
		return nil
	}
}

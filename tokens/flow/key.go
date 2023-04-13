package flow

import (
	"encoding/hex"
	"math/big"
)

const (
	// PubKeyBytesLenCompressed is compressed pubkey byte length
	PubKeyBytesLenCompressed = 33
	// PubKeyBytesLenUncompressed is uncompressed pubkey byte length
	PubKeyBytesLenUncompressed = 65
)

const (
	pubkeyCompressed byte = 0x2
)

// EcdsaPublic struct ripple ecdsa pubkey key
type EcdsaPublic struct {
	pub []byte
}

// Private not used
func (k *EcdsaPublic) Private(sequence *uint32) []byte {
	return nil
}

// Public returns pubkey bytes
func (k *EcdsaPublic) Public(sequence *uint32) []byte {
	if len(k.pub) == PubKeyBytesLenCompressed {
		return k.pub
	}
	xs := hex.EncodeToString(k.pub[1:33])
	ys := hex.EncodeToString(k.pub[33:])
	x, _ := new(big.Int).SetString(xs, 16)
	y, _ := new(big.Int).SetString(ys, 16)
	b := make([]byte, 0, PubKeyBytesLenCompressed)
	format := pubkeyCompressed
	if isOdd(y) {
		format |= 0x1
	}
	b = append(b, format)
	return paddedAppend(32, b, x.Bytes())
}

func isOdd(a *big.Int) bool {
	return a.Bit(0) == 1
}

func paddedAppend(size uint, dst, src []byte) []byte {
	for i := 0; i < int(size)-len(src); i++ {
		dst = append(dst, 0)
	}
	return append(dst, src...)
}

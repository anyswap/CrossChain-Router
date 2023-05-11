package starknet

import (
	"encoding/hex"
	"fmt"
	"hash"
	"math/big"
	"strings"

	"golang.org/x/crypto/sha3"
)

type KeccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

// ConvertSignature matches MPC signature with Starknet format
func ConvertSignature(r, s, v *big.Int) []string {
	var signature []string

	rl, rh := SplitUint256(r)
	sl, sh := SplitUint256(s)

	signature = append(signature, fmt.Sprintf("0x%s", v.Text(16)))
	signature = append(signature, fmt.Sprintf("0x%s", rl.Text(16)))
	signature = append(signature, fmt.Sprintf("0x%s", rh.Text(16)))
	signature = append(signature, fmt.Sprintf("0x%s", sl.Text(16)))
	signature = append(signature, fmt.Sprintf("0x%s", sh.Text(16)))

	return signature
}

// SplitUint256 splits a big.Int number with up-to 256-bit size into two 128-bit numbers
func SplitUint256(num *big.Int) (low, high *big.Int) {
	high = big.NewInt(0)
	low = big.NewInt(0)

	high.Rsh(num, 128)

	mask := big.NewInt(1)
	mask.Lsh(mask, 128).Sub(mask, big.NewInt(1))
	low.And(num, mask)

	return low, high
}

func SNValToBN(str string) *big.Int {
	if strings.Contains(str, "0x") {
		return HexToBN(str)
	} else {
		return StrToBig(str)
	}
}

func HexToBN(hexString string) *big.Int {
	numStr := strings.Replace(hexString, "0x", "", -1)

	n, _ := new(big.Int).SetString(numStr, 16)
	return n
}

func StrToBig(str string) *big.Int {
	b, _ := new(big.Int).SetString(str, 10)

	return b
}

func GetSelectorFromName(funcName string) *big.Int {
	kec := Keccak256([]byte(funcName))

	maskedKec := MaskBits(250, 8, kec)

	return new(big.Int).SetBytes(maskedKec)
}

func Keccak256(data ...[]byte) []byte {
	b := make([]byte, 32)
	d := NewKeccakState()
	for _, b := range data {
		d.Write(b)
	}
	_, err := d.Read(b)
	if err != nil {
		return nil
	}
	return b
}
func NewKeccakState() KeccakState {
	return sha3.NewLegacyKeccak256().(KeccakState)
}

func MaskBits(mask, wordSize int, slice []byte) (ret []byte) {
	excess := len(slice)*wordSize - mask
	for _, by := range slice {
		if excess > 0 {
			if excess > wordSize {
				excess = excess - wordSize
				continue
			}
			by <<= excess
			by >>= excess
			excess = 0
		}
		ret = append(ret, by)
	}
	return ret
}

func UTF8StrToBig(str string) *big.Int {
	hexStr := hex.EncodeToString([]byte(str))
	b, _ := new(big.Int).SetString(hexStr, 16)

	return b
}

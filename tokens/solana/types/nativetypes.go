// Copyright 2020 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mr-tron/base58"
)

// Padding bytes
type Padding []byte

// Hash alias of PublicKey
type Hash = PublicKey

// Signature 64 bytes
type Signature [64]byte

// NewSignatureFromBytes new signature from bytes
func NewSignatureFromBytes(in []byte) (out Signature, err error) {
	if len(in) != 64 {
		err = fmt.Errorf("invalid length, expected 64, got %d", len(in))
		return
	}
	copy(out[:], in)
	return
}

// NewSignatureFromString new signature from string
func NewSignatureFromString(in string) (out Signature, err error) {
	bytes, err := hex.DecodeString(in)
	if err != nil {
		return out, fmt.Errorf("hex decode: %w", err)
	}

	return NewSignatureFromBytes(bytes)
}

// MustSignatureFromString must signature from string
func MustSignatureFromString(in string) (out Signature) {
	out, err := NewSignatureFromString(in)
	if err != nil {
		panic(err)
	}
	return
}

// NewSignatureFromBase58 new signature from base58
func NewSignatureFromBase58(in string) (out Signature, err error) {
	bytes, err := base58.Decode(in)
	if err != nil {
		return out, fmt.Errorf("base58 decode: %w", err)
	}

	return NewSignatureFromBytes(bytes)
}

// ToSlice to slice
func (s Signature) ToSlice() []byte {
	return s[:]
}

// MarshalJSON json marshal
func (s Signature) MarshalJSON() ([]byte, error) {
	return json.Marshal(base58.Encode(s[:]))
}

// UnmarshalJSON json unmarshal
func (s *Signature) UnmarshalJSON(data []byte) (err error) {
	var str string
	err = json.Unmarshal(data, &str)
	if err != nil {
		return
	}

	dat, err := base58.Decode(str)
	if err != nil {
		return err
	}

	if len(dat) != 64 {
		return errors.New("invalid data length for public key")
	}

	target := Signature{}
	copy(target[:], dat)
	*s = target
	return
}

// Verify verify signature
func (s Signature) Verify(publicKey PublicKey, message []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(publicKey[:]), message, s[:])
}

func (s Signature) String() string {
	return base58.Encode(s[:])
}

// Base58 bytes
type Base58 []byte

// MarshalJSON json marshal
func (t Base58) MarshalJSON() ([]byte, error) {
	return json.Marshal(base58.Encode(t))
}

// UnmarshalJSON json unmarshal
func (t *Base58) UnmarshalJSON(data []byte) (err error) {
	var s string
	err = json.Unmarshal(data, &s)
	if err != nil {
		return
	}

	*t, err = base58.Decode(s)
	return
}

func (t Base58) String() string {
	return base58.Encode(t)
}

// Data bytes
type Data []byte

// MarshalJSON json marshal
func (t Data) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{base64.StdEncoding.EncodeToString(t), "base64"})
}

// UnmarshalJSON json unmarshal
func (t *Data) UnmarshalJSON(data []byte) (err error) {
	var in []string
	if err = json.Unmarshal(data, &in); err != nil {
		return err
	}

	if len(in) != 2 {
		return fmt.Errorf("invalid length for solana.Data, expected 2, found %d", len(in))
	}

	switch in[1] {
	case "base64":
		*t, err = base64.StdEncoding.DecodeString(in[0])
	default:
		return fmt.Errorf("unsupported encoding %s", in[1])
	}
	return
}

func (t Data) String() string {
	return base64.StdEncoding.EncodeToString(t)
}

// ByteWrapper type
type ByteWrapper struct {
	io.Reader
}

// ReadByte read byte
func (w *ByteWrapper) ReadByte() (byte, error) {
	var b [1]byte
	_, err := w.Read(b[:])
	return b[0], err
}

// Uint8Arr array of uint8
// solve json marshal will encode '[]uint8' to string
// rather than an array of numbers
type Uint8Arr []uint8

// MarshalJSON json marshal
func (t Uint8Arr) MarshalJSON() ([]byte, error) {
	var result string
	if t == nil {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", t)), ",")
	}
	return []byte(result), nil
}

func (t Uint8Arr) String() string {
	s, _ := json.Marshal(t)
	return string(s)
}

// BorshString borsh string
type BorshString struct {
	Length  uint32 `bin:"sizeof=Content"`
	Content []byte
}

// MarshalJSON json marshal
func (s BorshString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s.Content))
}

// UnmarshalJSON json unmarshal
func (s *BorshString) UnmarshalJSON(data []byte) (err error) {
	var in string
	if err = json.Unmarshal(data, &in); err != nil {
		return err
	}
	s.Length = uint32(len(in))
	s.Content = []byte(in)
	return nil
}

func (s *BorshString) String() string {
	return string(s.Content)
}

// ToBorshString to borsh encoded string
func ToBorshString(str string) BorshString {
	return BorshString{
		Length:  uint32(len(str)),
		Content: []byte(str),
	}
}

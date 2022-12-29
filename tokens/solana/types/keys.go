package types

import (
	"crypto"
	"crypto/ed25519"
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"filippo.io/edwards25519"
	"github.com/anyswap/CrossChain-Router/v3/tools"
	"github.com/mr-tron/base58"
)

// constants
const (
	MaxSeeds      = 16
	MaxSeedLength = 32

	PDAMarker = "ProgramDerivedAddress"
)

// known program IDs
var (
	TokenProgramID = MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	ATAProgramID   = MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
)

// PrivateKey bytes
type PrivateKey []byte

// MustPrivateKeyFromBase58 must decode private key from base58
func MustPrivateKeyFromBase58(in string) PrivateKey {
	out, err := PrivateKeyFromBase58(in)
	if err != nil {
		panic(err)
	}
	return out
}

// PrivateKeyFromBase58 decode private key from base58
func PrivateKeyFromBase58(privkey string) (PrivateKey, error) {
	res, err := base58.Decode(privkey)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// PrivateKeyFromSolanaKeygenFile decode private key from file
func PrivateKeyFromSolanaKeygenFile(file string) (PrivateKey, error) {
	content, err := tools.SafeReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read keygen file: %w", err)
	}

	var values []uint8
	err = json.Unmarshal(content, &values)
	if err != nil {
		return nil, fmt.Errorf("decode keygen file: %w", err)
	}

	return values, nil
}

func (k PrivateKey) String() string {
	return base58.Encode(k)
}

// NewRandomPrivateKey new random private key
func NewRandomPrivateKey() (PublicKey, PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(crypto_rand.Reader)
	if err != nil {
		return PublicKey{}, nil, err
	}
	var publicKey PublicKey
	copy(publicKey[:], pub)
	return publicKey, PrivateKey(priv), nil
}

// Sign sign message
func (k PrivateKey) Sign(payload []byte) (Signature, error) {
	p := ed25519.PrivateKey(k)
	signData, err := p.Sign(crypto_rand.Reader, payload, crypto.Hash(0))
	if err != nil {
		return Signature{}, err
	}

	var signature Signature
	copy(signature[:], signData)

	return signature, err
}

// PublicKey get public key
func (k PrivateKey) PublicKey() PublicKey {
	p := ed25519.PrivateKey(k)
	pub := p.Public().(ed25519.PublicKey)

	var publicKey PublicKey
	copy(publicKey[:], pub)

	return publicKey
}

// PublicKey bytes 32
type PublicKey [32]byte

// PublicKeyFromBytes get public key from bytes
func PublicKeyFromBytes(in []byte) (out PublicKey) {
	byteCount := len(in)
	if byteCount == 0 {
		return
	}

	max := 32
	if byteCount < max {
		max = byteCount
	}

	copy(out[:], in[0:max])
	return
}

// ToSlice to slice
func (p PublicKey) ToSlice() []byte {
	return p[:]
}

// MustPublicKeyFromBase58 must get public key from base58
func MustPublicKeyFromBase58(in string) PublicKey {
	out, err := PublicKeyFromBase58(in)
	if err != nil {
		panic(err)
	}
	return out
}

// PublicKeyFromBase58 get public key from base58
func PublicKeyFromBase58(in string) (out PublicKey, err error) {
	val, err := base58.Decode(in)
	if err != nil {
		return out, fmt.Errorf("decode: %w", err)
	}

	if len(val) != 32 {
		return out, fmt.Errorf("invalid length, expected 32, got %d", len(val))
	}

	copy(out[:], val)
	return
}

func checkSeeds(seeds [][]byte) error {
	if len(seeds) > MaxSeeds {
		return fmt.Errorf("max seeds count exceeded")
	}
	for _, seed := range seeds {
		if len(seed) > MaxSeedLength {
			return fmt.Errorf("max seed length exceeded")
		}
	}
	return nil
}

// PublicKeyFindProgramAddress create derived addresses
func PublicKeyFindProgramAddress(seeds [][]byte, programID PublicKey) (PublicKey, byte, error) {
	if err := checkSeeds(seeds); err != nil {
		return PublicKey{}, 0x00, err
	}
	for bump := uint8(255); bump > 0; bump-- {
		seedsWithBump := append(seeds, []byte{bump})
		key, err := createProgramAddress(seedsWithBump, programID)
		if err == nil {
			return key, bump, nil
		}
	}
	return PublicKey{}, 0x00, fmt.Errorf("unable to find a viable program address bump")
}

func createProgramAddress(seeds [][]byte, programID PublicKey) (PublicKey, error) {
	buf := make([]byte, 0, len(seeds)*MaxSeedLength+len(programID)+len(PDAMarker))
	for _, seed := range seeds {
		buf = append(buf, seed...)
	}
	buf = append(buf, programID[:]...)
	buf = append(buf, []byte(PDAMarker)...)
	pkey := sha256.Sum256(buf)
	if _, err := new(edwards25519.Point).SetBytes(pkey[:]); err == nil {
		return PublicKey{}, fmt.Errorf("invalid seeds, address must fall off the curve")
	}
	return pkey, nil
}

// FindAssociatedTokenAddress find associated token account
func FindAssociatedTokenAddress(walletAddress, tokenMintAddress PublicKey) (PublicKey, error) {
	pkey, _, err := PublicKeyFindProgramAddress(
		[][]byte{
			walletAddress.ToSlice(),
			TokenProgramID.ToSlice(),
			tokenMintAddress.ToSlice(),
		},
		ATAProgramID,
	)
	return pkey, err
}

// MarshalJSON json marshal
func (p PublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(base58.Encode(p[:]))
}

// UnmarshalJSON json unmarshal
func (p *PublicKey) UnmarshalJSON(data []byte) (err error) {
	var s string
	if err = json.Unmarshal(data, &s); err != nil {
		return err
	}

	*p, err = PublicKeyFromBase58(s)
	if err != nil {
		return fmt.Errorf("invalid public key %q: %w", s, err)
	}
	return
}

// Equals compare public key
func (p PublicKey) Equals(pb PublicKey) bool {
	return p == pb
}

var zeroPublicKey = PublicKey{}

// IsZero is zero public key
func (p PublicKey) IsZero() bool {
	return p == zeroPublicKey
}

func (p PublicKey) String() string {
	return base58.Encode(p[:])
}

// VerifySignature verify signature
func (p PublicKey) VerifySignature(message, sig []byte) bool {
	return ed25519.Verify(p[:], message, sig)
}

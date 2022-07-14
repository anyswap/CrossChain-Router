package near

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/mr-tron/base58"
)

// All supported key types
const (
	ED25519 = 0

	ED25519Prefix = "ed25519:"
)

// PublicKeyFromEd25519 derives a public key in NEAR encoding from pk.
func PublicKeyFromEd25519(pk ed25519.PublicKey) *PublicKey {
	var pubKey PublicKey
	pubKey.KeyType = ED25519
	copy(pubKey.Data[:], pk)
	return &pubKey
}

func (pk *PublicKey) Bytes() []byte {
	return pk.Data[:]
}

func (pk *PublicKey) Address() string {
	return hex.EncodeToString(pk.Bytes())
}

func (pk *PublicKey) String() string {
	var prefix string
	if pk.KeyType == ED25519 {
		prefix = ED25519Prefix
	}
	bs58Str := base58.Encode(pk.Bytes())
	return prefix + bs58Str
}

func PublicKeyFromHexString(pubKeyHex string) (*PublicKey, error) {
	pubKeyHex = strings.TrimPrefix(pubKeyHex, "0x")
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return nil, err
	}
	return PublicKeyFromBytes(pubKeyBytes)
}

func PublicKeyFromBytes(pubKeyBytes []byte) (*PublicKey, error) {
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return nil, errors.New("public key length is not equal 32")
	}
	return PublicKeyFromEd25519(ed25519.PublicKey(pubKeyBytes)), nil
}

func PublicKeyFromString(pub string) (*PublicKey, error) {
	pubKeyBytes, err := base58.Decode(strings.TrimPrefix(pub, ED25519Prefix))
	if err != nil {
		return nil, err
	}
	return PublicKeyFromBytes(pubKeyBytes)
}

func StringToPrivateKey(priv string) (*ed25519.PrivateKey, error) {
	privateKey, err := base58.Decode(strings.TrimPrefix(priv, ED25519Prefix))
	if err != nil {
		return nil, err
	}
	ed25519PriKey := ed25519.PrivateKey(privateKey)
	return &ed25519PriKey, nil
}

func GenerateKey() (seed, pub []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv.Seed(), pub, nil
}

func GeneratePubKeyBySeed(seed []byte) ([]byte, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, errors.New("seed length is not equal 32")
	}

	priv := ed25519.NewKeyFromSeed(seed)

	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("private key length is not equal 64")
	}

	pub := priv[32:]
	return pub, nil
}

func GeneratePubKeyByBase58(b58Key string) ([]byte, error) {
	seed, err := base58.Decode(b58Key)
	if err != nil {
		return nil, err
	}
	return GeneratePubKeyBySeed(seed)
}

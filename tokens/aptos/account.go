package aptos

import (
	"log"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos/tweetnacl"
	"golang.org/x/crypto/sha3"
)

type Account struct {
	KeyPair *tweetnacl.KeyPair
}

func NewAccount() *Account {
	keypair, err := tweetnacl.CryptoSignKeyPair()
	if err != nil {
		log.Fatal("NewAccount", "err", err)
	}
	return &Account{
		KeyPair: keypair,
	}
}

func NewAccountFromSeed(seedHex string) *Account {
	keypair, err := tweetnacl.CryptoSignKeyPairFromSeed(common.FromHex(seedHex))
	if err != nil {
		log.Fatal("CryptoSignKeyPair", "err", err)
	}
	return &Account{
		KeyPair: keypair,
	}
}

func NewAccountFromPubkey(pubkeyHex string) *Account {
	return &Account{
		KeyPair: &tweetnacl.KeyPair{PublicKey: common.Hex2Bytes(pubkeyHex), SecretKey: nil},
	}
}

func (account *Account) GetHexAddress() string {
	hash := sha3.New256()
	hash.Write(account.KeyPair.PublicKey)
	hash.Write([]byte("\x00"))
	return common.ToHex(hash.Sum(nil))
}

func (account *Account) GetPublicKeyHex() string {
	return common.ToHex(account.KeyPair.PublicKey)
}

func (account *Account) SignString(message string) (string, error) {
	signingMessage := message
	if common.HasHexPrefix(signingMessage) {
		signingMessage = signingMessage[2:]
	}
	signature, err := tweetnacl.CryptoSign(common.Hex2Bytes(signingMessage), account.KeyPair.SecretKey)
	if err != nil {
		return "", err
	}
	return common.ToHex(signature[:64]), nil
}

func (account *Account) SignBytes(message []byte) (string, error) {
	signature, err := tweetnacl.CryptoSign(message, account.KeyPair.SecretKey)
	if err != nil {
		return "", err
	}
	return common.ToHex(signature[:64]), nil
}
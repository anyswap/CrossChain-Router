package types

import (
	"fmt"
)

// Account account type
type Account struct {
	PrivateKey PrivateKey
}

// NewAccount new account
func NewAccount() *Account {
	_, privateKey, err := NewRandomPrivateKey()
	if err != nil {
		panic(fmt.Sprintf("failed to generate private key: %s", err))
	}
	return &Account{
		PrivateKey: privateKey,
	}
}

// AccountFromPrivateKeyBase58 account from private key base58
func AccountFromPrivateKeyBase58(privateKey string) (*Account, error) {
	k, err := PrivateKeyFromBase58(privateKey)
	if err != nil {
		return nil, fmt.Errorf("account from private key: private key from b58: %w", err)
	}
	return &Account{
		PrivateKey: k,
	}, nil
}

// PublicKey get account public key
func (a *Account) PublicKey() PublicKey {
	return a.PrivateKey.PublicKey()
}

// AccountSettable interface
type AccountSettable interface {
	SetAccounts(accounts []*AccountMeta) error
}

// AccountMeta account meta type
type AccountMeta struct {
	PublicKey  PublicKey
	IsSigner   bool
	IsWritable bool
}

func (a *AccountMeta) less(act *AccountMeta) bool {
	if a.IsSigner != act.IsSigner {
		return a.IsSigner
	}
	if a.IsWritable != act.IsWritable {
		return a.IsWritable
	}
	return false
}

package starknet

import (
	"math/big"

	"github.com/dontpanicdao/caigo"
	"github.com/dontpanicdao/caigo/types"
)

var MAXFEE, _ = big.NewInt(0).SetString("0x20000000000", 0)

const DefaultAccountVersion = 1

type IAccount interface {
	Sign(msgHash *big.Int, privKey string) (*big.Int, *big.Int, error)
}

type AccountPlugin interface {
	PluginCall(calls []FunctionCall) (FunctionCall, error)
}

type Account struct {
	chainId string
	Address string
	private string
	version uint64
}

func NewAccountWithPrivateKey(private, address, chainID string) (*Account, error) {
	version := uint64(DefaultAccountVersion)
	return &Account{
		Address: address,
		private: private,
		chainId: chainID,
		version: version,
	}, nil
}

func NewAccount(address, chainID string) (*Account, error) {
	version := uint64(DefaultAccountVersion)
	return &Account{
		Address: address,
		chainId: chainID,
		version: version,
	}, nil
}

func (account *Account) Sign(msgHash *big.Int) (*big.Int, *big.Int, error) {
	return caigo.Curve.Sign(msgHash, types.HexToBN(account.private))
}

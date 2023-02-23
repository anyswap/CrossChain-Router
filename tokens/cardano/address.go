package cardano

import (
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/btcsuite/btcutil/bech32"
	"github.com/echovl/cardano-go"
	cardanosdk "github.com/echovl/cardano-go"
	"github.com/echovl/cardano-go/crypto"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(addr string) bool {
	if _, err := cardanosdk.NewAddress(addr); err != nil {
		return false
	}
	return true
}

// PublicKeyToAddress impl
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	pubStr, err := bech32.EncodeFromBase256("addr_vk", common.FromHex(pubKeyHex))
	if err != nil {
		return "", err
	}
	pubKey, err := crypto.NewPubKey(pubStr)
	if err != nil {
		return "", err
	}
	if pubKeyHex != pubKey.String() && pubKeyHex != "0x"+pubKey.String() {
		return "", errors.New("pubKey not match")
	}
	payment, err := cardanosdk.NewKeyCredential(pubKey)
	if err != nil {
		return "", err
	}
	network := cardanosdk.Mainnet
	if b.GetChainConfig().GetChainID().Cmp(GetStubChainID(testnetNetWork)) == 0 {
		network = cardanosdk.Testnet
	}
	enterpriseAddr, err := cardano.NewEnterpriseAddress(network, payment)
	if err != nil {
		return "", err
	}
	return enterpriseAddr.String(), nil
}

// VerifyMPCPubKey verify mpc address and public key is matching
func (b *Bridge) VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	addr, err := b.PublicKeyToAddress(mpcPubkey)
	if err != nil {
		return err
	}
	if addr == mpcAddress {
		return nil
	}
	return errors.New(fmt.Sprint("addr not match ", "derivedAddr", addr, "Addr", mpcAddress))
}

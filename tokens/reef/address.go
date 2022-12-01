package reef

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

// const mpc_publickey_type = "sr25519"

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	if !common.IsHexAddress(address) {
		return false
	}
	unprefixedHex, ok, hasUpperChar := common.GetUnprefixedHex(address)
	if hasUpperChar {
		if strings.ToUpper(address) == address {
			return true
		}
		// valid checksum
		if unprefixedHex != common.HexToAddress(address).Hex()[2:] {
			return false
		}
	}
	return ok
}

func AddressToPubkey(base58Address string) []byte {
	addrBytes, _ := base58.Decode(base58Address)
	return addrBytes[1 : len(addrBytes)-2]
}

// publicKey to evmAddress by reef default
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	content := []byte("evm:")
	content = append(content, common.FromHex(pubKey)...)

	blake2AsU8a := blake2b.Sum256(content)
	// return Public2address(mpc_publickey_type, pubKey)
	return common.ToHex(blake2AsU8a[0:20]), nil
}

// implement EvmContractBridge VerifyMPCPubKey
// mpcAddress maybe is pair of mpcPubkey
// or just calc by reef algorithem
func (b *Bridge) VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	if !common.IsHexAddress(mpcAddress) {
		return fmt.Errorf("wrong mpc address '%v'", mpcAddress)
	}
	// evm check
	pubkeyAddr, err := b.Bridge.PublicKeyToAddress(mpcPubkey)
	if err != nil {
		pubkeyAddr, err = b.PublicKeyToAddress(mpcPubkey)
		if err != nil {
			return err
		}
	}
	if !strings.EqualFold(pubkeyAddr, mpcAddress) {
		return fmt.Errorf("mpc address %v and public key address %v is not match", mpcAddress, pubkeyAddr)
	}
	return nil
}

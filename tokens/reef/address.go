package reef

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

const MPC_PUBLICKEY_TYPE = "SR25519"
const encode = "SS58PRE"

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

// pubkey to reef address
func PubkeyToReefAddress(publicKey string) string {
	encode := []byte(encode)
	input := []byte{byte(42)}
	if common.HasHexPrefix(publicKey) {
		publicKey = publicKey[2:]
	}
	input = append(input, common.Hex2Bytes(publicKey)...)
	blake2AsU8a := blake2b.Sum512(append(encode, input...))

	input = append(input, blake2AsU8a[0:2]...)

	base58Address := base58.Encode(input)
	return base58Address
}

// reef address to pubkey
func AddressToPubkey(base58Address string) []byte {
	addrBytes, _ := base58.Decode(base58Address)
	if len(addrBytes) <= 0 {
		return nil
	}
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
	reefAddr := PubkeyToReefAddress(mpcPubkey)

	reefEvmAddr, err := b.QueryEvmAddress(reefAddr)
	if err != nil {
		return err
	}

	if !strings.EqualFold(reefEvmAddr.LowerHex(), mpcAddress) {
		return fmt.Errorf("mpc address %v and public key reefAddr %v evmaddress %v is not match", mpcAddress, reefAddr, reefEvmAddr.LowerHex())
	}
	return nil
}

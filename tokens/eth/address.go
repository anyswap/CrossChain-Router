package eth

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
)

var (
	eip1167Proxies        = make(map[common.Address]common.Address) // proxy -> master
	maxEip1167ProxiesSize = 10000

	eip1167ProxyCodePattern = regexp.MustCompile("^0x363d3d373d3d3d363d73([0-9a-fA-F]{40})5af43d82803e903d91602b57fd5bf3$")
	eip1167ProxyCodeLen     = 45 // bytes

	contractCodeHashes    = make(map[common.Address]common.Hash)
	maxContractCodeHashes = 2000
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	if !common.IsHexAddress(address) {
		return false
	}
	if b.DontCheckAddressMixedCase {
		return true
	}
	unprefixedHex, ok, hasUpperChar := common.GetUnprefixedHex(address)
	if hasUpperChar {
		// valid checksum
		if unprefixedHex != common.HexToAddress(address).Hex()[2:] {
			return false
		}
	}
	return ok
}

// IsContractAddress is contract address
func (b *Bridge) IsContractAddress(address string) (bool, error) {
	code, err := b.getContractCode(address)
	if err == nil {
		return len(code) > 1, nil // unexpect RSK getCode return 0x00
	}
	return false, err
}

// GetEIP1167Master get eip1167 master address
func (b *Bridge) GetEIP1167Master(proxy common.Address) (master common.Address) {
	master, exist := eip1167Proxies[proxy]
	if exist {
		return master
	}
	if len(eip1167Proxies) > maxEip1167ProxiesSize {
		eip1167Proxies = make(map[common.Address]common.Address) // clear
	}

	proxyAddr := proxy.String()

	code, err := b.getContractCode(proxyAddr)
	if err != nil || len(code) != eip1167ProxyCodeLen {
		return master
	}

	matches := eip1167ProxyCodePattern.FindStringSubmatch(common.ToHex(code))
	if len(matches) == 2 {
		master = common.HexToAddress(matches[1])
		eip1167Proxies[proxy] = master
	}
	return master
}

// GetContractCodeHash get contract code hash
func (b *Bridge) GetContractCodeHash(contract common.Address) common.Hash {
	codeHash, exist := contractCodeHashes[contract]
	if exist {
		return codeHash
	}
	if len(contractCodeHashes) > maxContractCodeHashes {
		contractCodeHashes = make(map[common.Address]common.Hash) // clear
	}

	code, err := b.getContractCode(contract.String())
	if err == nil && len(code) > 1 {
		codeHash = common.Keccak256Hash(code)
		contractCodeHashes[contract] = codeHash
	}
	return codeHash
}

func (b *Bridge) getContractCode(contract string) (code []byte, err error) {
	for i := 0; i < retryRPCCount; i++ {
		code, err = b.GetCode(contract)
		if err == nil && len(code) > 1 {
			return code, nil
		}
		if err != nil {
			log.Warn("get contract code failed", "contract", contract, "err", err)
		}
		time.Sleep(retryRPCInterval)
	}
	return code, err
}

// VerifyMPCPubKey verify mpc address and public key is matching
func VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	if !common.IsHexAddress(mpcAddress) {
		return fmt.Errorf("wrong mpc address '%v'", mpcAddress)
	}
	pkBytes := common.FromHex(mpcPubkey)
	if len(pkBytes) != 65 || pkBytes[0] != 4 {
		return fmt.Errorf("wrong mpc public key '%v'", mpcPubkey)
	}
	pubKey := ecdsa.PublicKey{
		Curve: crypto.S256(),
		X:     new(big.Int).SetBytes(pkBytes[1:33]),
		Y:     new(big.Int).SetBytes(pkBytes[33:65]),
	}
	pubAddr := crypto.PubkeyToAddress(pubKey)
	if !strings.EqualFold(pubAddr.String(), mpcAddress) {
		return fmt.Errorf("mpc address %v and public key address %v is not match", mpcAddress, pubAddr.String())
	}
	return nil
}

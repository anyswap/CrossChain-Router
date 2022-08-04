package tron

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"

	tronaddress "github.com/fbsobreira/gotron-sdk/pkg/address"
)

var (
	eip1167Proxies        = make(map[string]common.Address) // proxy -> master
	maxEip1167ProxiesSize = 10000

	eip1167ProxyCodePattern = regexp.MustCompile("^0x363d3d373d3d3d363d73([0-9a-fA-F]{40})5af43d82803e903d91602b57fd5bf3$")
	eip1167ProxyCodeLen     = 45 // bytes

	contractCodeHashes    = make(map[string]common.Hash)
	maxContractCodeHashes = 2000
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	if common.IsHexAddress(address) {
		return true
	}
	if len(address) != tronaddress.AddressLengthBase58 {
		return false
	}
	addr, err := tronaddress.Base58ToAddress(address)
	if err != nil || len(addr) != tronaddress.AddressLength {
		return false
	}
	return true
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
func (b *Bridge) GetEIP1167Master(proxy string) (master common.Address) {
	master, exist := eip1167Proxies[proxy]
	if exist {
		return master
	}
	if len(eip1167Proxies) > maxEip1167ProxiesSize {
		eip1167Proxies = make(map[string]common.Address) // clear
	}

	code, err := b.getContractCode(proxy)
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
func (b *Bridge) GetContractCodeHash(contract string) common.Hash {
	codeHash, exist := contractCodeHashes[contract]
	if exist {
		return codeHash
	}
	if len(contractCodeHashes) > maxContractCodeHashes {
		contractCodeHashes = make(map[string]common.Hash) // clear
	}

	code, err := b.getContractCode(contract)
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
	pkAddress, err := pubKeyToAddress(mpcPubkey)
	if err != nil {
		return err
	}
	if !strings.EqualFold(pkAddress, mpcAddress) {
		return fmt.Errorf("mpc address %v and public key address %v is not match", mpcAddress, pkAddress)
	}
	return nil
}

func ethToTron(ethAddress string) (string, error) {
	if !common.IsHexAddress(ethAddress) {
		return "", fmt.Errorf("wrong eth address: %v", ethAddress)
	}
	bz := common.HexToAddress(ethAddress).Bytes()
	bz = append([]byte{tronaddress.TronBytePrefix}, bz...)
	return tronaddress.Address(bz).String(), nil
}

func tronToEth(tronAddress string) (string, error) {
	if len(tronAddress) != tronaddress.AddressLengthBase58 {
		return "", fmt.Errorf("wrong tron address: %v", tronAddress)
	}
	addr, err := tronaddress.Base58ToAddress(tronAddress)
	if err != nil || len(addr) != tronaddress.AddressLength {
		return "", fmt.Errorf("wrong tron address: %v", tronAddress)
	}
	ethaddr := common.BytesToAddress(addr.Bytes()[1:])
	return ethaddr.LowerHex(), nil
}

func convertToTronAddress(bs []byte) string {
	ethAddress := common.BytesToAddress(bs).LowerHex()
	tronAddress, _ := ethToTron(ethAddress)
	return tronAddress
}

func convertToEthAddress(tronAddress string) string {
	ethAddress, err := tronToEth(tronAddress)
	if err != nil {
		log.Error("convert tron address to eth address failed", "tronAddress", tronAddress, "err", err)
	}
	return ethAddress
}

func pubKeyToAddress(pubKeyHex string) (address string, err error) {
	pubKeyHex = strings.TrimPrefix(pubKeyHex, "0x")
	bz, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return "", err
	}
	ecdsaPub, err := crypto.UnmarshalPubkey(bz)
	if err != nil {
		return "", err
	}
	return tronaddress.PubkeyToAddress(*ecdsaPub).String(), nil
}

// PublicKeyToAddress returns cosmos public key address
func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (address string, err error) {
	return pubKeyToAddress(pubKeyHex)
}

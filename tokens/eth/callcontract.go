package eth

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
)

// token types (should be all upper case)
const (
	ERC20TokenType = "ERC20"
)

var erc20CodeParts = map[string][]byte{
	"name":         common.FromHex("0x06fdde03"),
	"symbol":       common.FromHex("0x95d89b41"),
	"decimals":     common.FromHex("0x313ce567"),
	"totalSupply":  common.FromHex("0x18160ddd"),
	"balanceOf":    common.FromHex("0x70a08231"),
	"transfer":     common.FromHex("0xa9059cbb"),
	"transferFrom": common.FromHex("0x23b872dd"),
	"approve":      common.FromHex("0x095ea7b3"),
	"allowance":    common.FromHex("0xdd62ed3e"),
	"LogTransfer":  common.FromHex("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
	"LogApproval":  common.FromHex("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"),
}

// GetErc20TotalSupply get erc20 total supply of address
func (b *Bridge) GetErc20TotalSupply(contract string) (*big.Int, error) {
	data := make(hexutil.Bytes, 4)
	copy(data[:4], erc20CodeParts["totalSupply"])
	result, err := b.CallContract(contract, data, "latest")
	if err != nil {
		return nil, err
	}
	return common.GetBigIntFromStr(result)
}

// GetErc20Balance get erc20 balacne of address
func (b *Bridge) GetErc20Balance(contract, address string) (*big.Int, error) {
	data := make(hexutil.Bytes, 36)
	copy(data[:4], erc20CodeParts["balanceOf"])
	copy(data[4:], common.HexToAddress(address).Hash().Bytes())
	result, err := b.CallContract(contract, data, "latest")
	if err != nil {
		return nil, err
	}
	return common.GetBigIntFromStr(result)
}

// GetErc20Decimals get erc20 decimals
func (b *Bridge) GetErc20Decimals(contract string) (uint8, error) {
	data := make(hexutil.Bytes, 4)
	copy(data[:4], erc20CodeParts["decimals"])
	result, err := b.CallContract(contract, data, "latest")
	if err != nil {
		return 0, err
	}
	decimals, err := common.GetUint64FromStr(result)
	return uint8(decimals), err
}

// GetTokenBalance api
func (b *Bridge) GetTokenBalance(tokenType, tokenAddress, accountAddress string) (*big.Int, error) {
	switch strings.ToUpper(tokenType) {
	case ERC20TokenType:
		return b.GetErc20Balance(tokenAddress, accountAddress)
	default:
		return nil, fmt.Errorf("[%v] can not get token balance of token with type '%v'", b.ChainConfig.BlockChain, tokenType)
	}
}

// GetTokenSupply impl
func (b *Bridge) GetTokenSupply(tokenType, tokenAddress string) (*big.Int, error) {
	switch strings.ToUpper(tokenType) {
	case ERC20TokenType:
		return b.GetErc20TotalSupply(tokenAddress)
	default:
		return nil, fmt.Errorf("[%v] can not get token supply of token with type '%v'", b.ChainConfig.BlockChain, tokenType)
	}
}

// GetFactoryAddress call "factory()"
func (b *Bridge) GetFactoryAddress(contractAddr string) (string, error) {
	data := common.FromHex("0xc45a0155")
	res, err := b.CallContract(contractAddr, data, "latest")
	if err != nil {
		return "", err
	}
	return common.BytesToAddress(common.GetData(common.FromHex(res), 0, 32)).LowerHex(), nil
}

// GetWNativeAddress call "wNATIVE()"
func (b *Bridge) GetWNativeAddress(contractAddr string) (string, error) {
	data := common.FromHex("0x8fd903f5")
	res, err := b.CallContract(contractAddr, data, "latest")
	if err != nil {
		return "", err
	}
	return common.BytesToAddress(common.GetData(common.FromHex(res), 0, 32)).LowerHex(), nil
}

// GetUnderlyingAddress call "underlying()"
func (b *Bridge) GetUnderlyingAddress(contractAddr string) (string, error) {
	data := common.FromHex("0x6f307dc3")
	res, err := b.CallContract(contractAddr, data, "latest")
	if err != nil {
		return "", err
	}
	return common.BytesToAddress(common.GetData(common.FromHex(res), 0, 32)).LowerHex(), nil
}

// GetMPCAddress call "mpc()"
func (b *Bridge) GetMPCAddress(contractAddr string) (string, error) {
	data := common.FromHex("0xf75c2664")
	res, err := b.CallContract(contractAddr, data, "latest")
	if err != nil {
		return "", err
	}
	return common.BytesToAddress(common.GetData(common.FromHex(res), 0, 32)).LowerHex(), nil
}

// GetVaultAddress call "vault()"
func (b *Bridge) GetVaultAddress(contractAddr string) (string, error) {
	data := common.FromHex("0xfbfa77cf")
	res, err := b.CallContract(contractAddr, data, "latest")
	if err != nil {
		return "", err
	}
	return common.BytesToAddress(common.GetData(common.FromHex(res), 0, 32)).LowerHex(), nil
}

// GetOwnerAddress call "owner()"
func (b *Bridge) GetOwnerAddress(contractAddr string) (string, error) {
	data := common.FromHex("0x8da5cb5b")
	res, err := b.CallContract(contractAddr, data, "latest")
	if err != nil {
		return "", err
	}
	return common.BytesToAddress(common.GetData(common.FromHex(res), 0, 32)).LowerHex(), nil
}

// IsMinter call "isMinter(address)"
func (b *Bridge) IsMinter(contractAddr, minterAddr string) (bool, error) {
	funcHash := common.FromHex("0xaa271e1a")
	data := make([]byte, 36)
	copy(data[:4], funcHash)
	copy(data[4:36], common.HexToAddress(minterAddr).Hash().Bytes())
	res, err := b.CallContract(contractAddr, data, "latest")
	if err != nil {
		return false, err
	}
	return common.GetBigInt(common.FromHex(res), 0, 32).Sign() != 0, nil
}

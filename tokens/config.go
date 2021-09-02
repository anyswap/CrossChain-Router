package tokens

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
)

// ChainConfig struct
type ChainConfig struct {
	ChainID        string
	BlockChain     string
	RouterContract string
	Confirmations  uint64
	InitialHeight  uint64

	// cached value
	routerMPC       string
	routerMPCPubkey string
	routerFactory   string
	routerWNative   string
}

// TokenConfig struct
type TokenConfig struct {
	TokenID         string
	Decimals        uint8
	ContractAddress string
	ContractVersion uint64

	// calced value
	underlying common.Address
}

// SwapConfig struct
type SwapConfig struct {
	MaximumSwap           *big.Int
	MinimumSwap           *big.Int
	BigValueThreshold     *big.Int
	SwapFeeRatePerMillion uint64
	MaximumSwapFee        *big.Int
	MinimumSwapFee        *big.Int
}

// GatewayConfig struct
type GatewayConfig struct {
	APIAddress    []string
	APIAddressExt []string
}

// CheckConfig check chain config
func (c *ChainConfig) CheckConfig() error {
	if c.BlockChain == "" {
		return errors.New("chain must config 'BlockChain'")
	}
	if c.ChainID == "" {
		return errors.New("chain must config 'ChainID'")
	}
	if _, ok := new(big.Int).SetString(c.ChainID, 0); !ok {
		return errors.New("chain with wrong 'ChainID'")
	}
	if c.Confirmations == 0 {
		return errors.New("chain must config nonzero 'Confirmations'")
	}
	if c.RouterContract == "" {
		return errors.New("chain must config 'RouterContract'")
	}
	return nil
}

// SetRouterMPC set router mpc
func (c *ChainConfig) SetRouterMPC(mpc string) {
	c.routerMPC = mpc
}

// GetRouterMPC get router mpc
func (c *ChainConfig) GetRouterMPC() string {
	return c.routerMPC
}

// SetRouterMPCPubkey set router mpc public key
func (c *ChainConfig) SetRouterMPCPubkey(pubkey string) {
	c.routerMPCPubkey = pubkey
}

// GetRouterMPCPubkey get router mpc public key
func (c *ChainConfig) GetRouterMPCPubkey() string {
	return c.routerMPCPubkey
}

// SetRouterFactory set factory address of router contract
func (c *ChainConfig) SetRouterFactory(factory string) {
	c.routerFactory = factory
}

// GetRouterFactory get factory address of router contract
func (c *ChainConfig) GetRouterFactory() string {
	return c.routerFactory
}

// SetRouterWNative set wNative address of router contract
func (c *ChainConfig) SetRouterWNative(wNative string) {
	c.routerWNative = wNative
}

// GetRouterWNative get wNative address of router contract
func (c *ChainConfig) GetRouterWNative() string {
	return c.routerWNative
}

// CheckConfig check token config
func (c *TokenConfig) CheckConfig() error {
	if c.TokenID == "" {
		return errors.New("token must config 'TokenID'")
	}
	if c.ContractAddress == "" {
		return errors.New("token must config 'ContractAddress'")
	}
	if params.IsNFTRouter() {
		if c.Decimals != 0 {
			return errors.New("NFT token must config 'Decimals' to 0")
		}
	}
	return nil
}

// SetUnderlying set underlying
func (c *TokenConfig) SetUnderlying(underlying common.Address) {
	c.underlying = underlying
}

// GetUnderlying get underlying
func (c *TokenConfig) GetUnderlying() common.Address {
	return c.underlying
}

// CheckConfig check swap config
func (c *SwapConfig) CheckConfig() error {
	if c.MaximumSwap == nil || c.MaximumSwap.Sign() <= 0 {
		return errors.New("token must config 'MaximumSwap' (positive)")
	}
	if c.MinimumSwap == nil || c.MinimumSwap.Sign() <= 0 {
		return errors.New("token must config 'MinimumSwap' (positive)")
	}
	if c.MinimumSwap.Cmp(c.MaximumSwap) > 0 {
		return errors.New("wrong token config, MinimumSwap > MaximumSwap")
	}
	if c.BigValueThreshold == nil || c.BigValueThreshold.Sign() <= 0 {
		return errors.New("token must config 'BigValueThreshold' (positive)")
	}
	if c.SwapFeeRatePerMillion >= 1000000 {
		return errors.New("token must config 'SwapFeeRatePerMillion' (< 1000000)")
	}
	if c.MaximumSwapFee == nil || c.MaximumSwapFee.Sign() < 0 {
		return errors.New("token must config 'MaximumSwapFee' (non-negative)")
	}
	if c.MinimumSwapFee == nil || c.MinimumSwapFee.Sign() < 0 {
		return errors.New("token must config 'MinimumSwapFee' (non-negative)")
	}
	if c.MinimumSwapFee.Cmp(c.MaximumSwapFee) > 0 {
		return errors.New("wrong token config, MinimumSwapFee > MaximumSwapFee")
	}
	if c.MinimumSwap.Cmp(c.MinimumSwapFee) < 0 {
		return errors.New("wrong token config, MinimumSwap < MinimumSwapFee")
	}
	if c.SwapFeeRatePerMillion == 0 && c.MinimumSwapFee.Sign() > 0.0 {
		return errors.New("wrong token config, MinimumSwapFee should be 0 if SwapFeeRatePerMillion is 0")
	}
	return nil
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

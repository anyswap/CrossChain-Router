package tokens

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

const (
	MaxStandardTokenVersion = uint64(10000)
	MinWrapperTokenVersion  = uint64(20000)
)

// ChainConfig struct
type ChainConfig struct {
	ChainID        string
	BlockChain     string
	RouterContract string
	Confirmations  uint64
	InitialHeight  uint64

	// cached value
	chainID *big.Int
}

// TokenConfig struct
type TokenConfig struct {
	TokenID         string
	Decimals        uint8
	ContractAddress string
	ContractVersion uint64
	RouterContract  string

	// calced value
	underlying string

	Checked bool `json:"-"`
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
	APIAddress         []string
	APIAddressExt      []string `json:",omitempty"`
	FinalizeAPIAddress []string `json:",omitempty"`
}

// CheckConfig check chain config
func (c *ChainConfig) CheckConfig() (err error) {
	if c.BlockChain == "" {
		return errors.New("chain must config 'BlockChain'")
	}
	if c.ChainID == "" {
		return errors.New("chain must config 'ChainID'")
	}
	c.chainID, err = common.GetBigIntFromStr(c.ChainID)
	if err != nil {
		return fmt.Errorf("chain with wrong 'ChainID'. %w", err)
	}
	if c.Confirmations == 0 {
		return errors.New("chain must config nonzero 'Confirmations'")
	}
	if c.RouterContract == "" {
		return errors.New("chain must config 'RouterContract'")
	}
	return nil
}

// GetChainID get chainID of number
func (c *ChainConfig) GetChainID() *big.Int {
	return c.chainID
}

// CheckConfig check token config
func (c *TokenConfig) CheckConfig() error {
	if c.TokenID == "" {
		return errors.New("token must config 'TokenID'")
	}
	if c.ContractAddress == "" {
		return errors.New("token must config 'ContractAddress'")
	}
	if !IsERC20Router() && c.Decimals != 0 {
		return errors.New("non ERC20 token must config 'Decimals' to 0")
	}
	return nil
}

// IsStandardTokenVersion is standard token version
func (c *TokenConfig) IsStandardTokenVersion() bool {
	return c.ContractVersion > 0 && c.ContractVersion <= MaxStandardTokenVersion
}

// IsWrapperTokenVersion is wrapper token version
func (c *TokenConfig) IsWrapperTokenVersion() bool {
	return c.ContractVersion >= MinWrapperTokenVersion
}

// SetUnderlying set underlying
func (c *TokenConfig) SetUnderlying(underlying string) {
	c.underlying = underlying
}

// GetUnderlying get underlying
func (c *TokenConfig) GetUnderlying() string {
	return c.underlying
}

// CheckConfig check swap config
//nolint:funlen,gocyclo // ok
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

package tokens

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

// token version boundaries
const (
	MaxStandardTokenVersion = uint64(10000)
	MinWrapperTokenVersion  = uint64(20000)
)

// ChainConfig struct
type ChainConfig struct {
	ChainID        string
	BlockChain     string
	RouterContract string
	RouterVersion  string
	Confirmations  uint64
	InitialHeight  uint64
	Extra          string

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
	RouterVersion   string
	Extra           string

	// calced value
	underlying string

	Checked bool `json:"-"`
}

// SwapConfig struct
type SwapConfig struct {
	MaximumSwap       *big.Int
	MinimumSwap       *big.Int
	BigValueThreshold *big.Int
}

// FeeConfig struct
type FeeConfig struct {
	MaximumSwapFee        *big.Int
	MinimumSwapFee        *big.Int
	SwapFeeRatePerMillion uint64
}

// GatewayConfig struct
type GatewayConfig struct {
	APIAddress         []string
	APIAddressExt      []string `json:",omitempty"`
	EVMAPIAddress      []string `json:",omitempty"`
	FinalizeAPIAddress []string `json:",omitempty"`
	GRPCAPIAddress     []string `json:",omitempty"`

	// internal usage
	AdjustContext interface{} `toml:"-" json:"-"`

	// adjust gateway related
	OriginAllGatewayURLs []string `toml:"-" json:"-"`
	AllGatewayURLs       []string `toml:"-" json:"-"`
}

// IsEmpty is not configed
func (c *GatewayConfig) IsEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.APIAddress) == 0 &&
		len(c.EVMAPIAddress) == 0 &&
		len(c.GRPCAPIAddress) == 0
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
	if c.MinimumSwap == nil || c.MinimumSwap.Sign() < 0 {
		return errors.New("token must config 'MinimumSwap' (non-negative)")
	}
	if c.BigValueThreshold == nil || c.BigValueThreshold.Sign() <= 0 {
		return errors.New("token must config 'BigValueThreshold' (positive)")
	}
	if c.BigValueThreshold.Cmp(c.MaximumSwap) > 0 {
		return errors.New("wrong token config, BigValueThreshold > MaximumSwap")
	}
	if c.MinimumSwap.Cmp(c.BigValueThreshold) > 0 {
		return errors.New("wrong token config, MinimumSwap > BigValueThreshold")
	}
	return nil
}

// CheckConfig check fee config
func (c *FeeConfig) CheckConfig() error {
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
	if c.SwapFeeRatePerMillion == 0 && c.MinimumSwapFee.Sign() > 0.0 {
		return errors.New("wrong token config, MinimumSwapFee should be 0 if SwapFeeRatePerMillion is 0")
	}
	return nil
}

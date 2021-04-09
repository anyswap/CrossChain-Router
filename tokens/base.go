package tokens

import (
	"math/big"
	"strings"

	cmath "github.com/anyswap/CrossChain-Router/common/math"
)

// CrossChainBridgeBase base bridge
type CrossChainBridgeBase struct {
	ChainConfig    *ChainConfig
	GatewayConfig  *GatewayConfig
	TokenConfigMap map[string]*TokenConfig // key is token address
}

// NewCrossChainBridgeBase new base bridge
func NewCrossChainBridgeBase() *CrossChainBridgeBase {
	return &CrossChainBridgeBase{
		TokenConfigMap: make(map[string]*TokenConfig),
	}
}

// SetChainConfig set chain config
func (b *CrossChainBridgeBase) SetChainConfig(chainCfg *ChainConfig) {
	b.ChainConfig = chainCfg
}

// SetGatewayConfig set gateway config
func (b *CrossChainBridgeBase) SetGatewayConfig(gatewayCfg *GatewayConfig) {
	b.GatewayConfig = gatewayCfg
}

// SetTokenConfig set token config
func (b *CrossChainBridgeBase) SetTokenConfig(token string, tokenCfg *TokenConfig) {
	b.TokenConfigMap[strings.ToLower(token)] = tokenCfg
}

// RemoveTokenConfig remove token config
func (b *CrossChainBridgeBase) RemoveTokenConfig(token string) {
	b.TokenConfigMap[strings.ToLower(token)] = nil
}

// GetChainConfig get chain config
func (b *CrossChainBridgeBase) GetChainConfig() *ChainConfig {
	return b.ChainConfig
}

// GetGatewayConfig get gateway config
func (b *CrossChainBridgeBase) GetGatewayConfig() *GatewayConfig {
	return b.GatewayConfig
}

// GetTokenConfig get token config
func (b *CrossChainBridgeBase) GetTokenConfig(token string) *TokenConfig {
	return b.TokenConfigMap[strings.ToLower(token)]
}

// GetBigValueThreshold get big value threshold
func (b *CrossChainBridgeBase) GetBigValueThreshold(token string) *big.Int {
	return b.GetTokenConfig(token).BigValueThreshold
}

// CheckTokenSwapValue check swap value is in right range
func CheckTokenSwapValue(token *TokenConfig, value *big.Int) bool {
	if value == nil {
		return false
	}
	if value.Cmp(token.MinimumSwap) < 0 {
		return false
	}
	if value.Cmp(token.MaximumSwap) > 0 {
		return false
	}
	swappedValue := CalcSwapValue(token, value)
	return swappedValue.Sign() > 0
}

// CalcSwapValue calc swap value (get rid of fee)
func CalcSwapValue(token *TokenConfig, value *big.Int) *big.Int {
	if token.SwapFeeRatePerMillion == 0 {
		return value
	}

	swapFee := new(big.Int).Mul(value, new(big.Int).SetUint64(token.SwapFeeRatePerMillion))
	swapFee.Div(swapFee, big.NewInt(1000000))

	if swapFee.Cmp(token.MinimumSwapFee) < 0 {
		swapFee = token.MinimumSwapFee
	} else if swapFee.Cmp(token.MaximumSwapFee) > 0 {
		swapFee = token.MaximumSwapFee
	}

	if value.Cmp(swapFee) > 0 {
		return new(big.Int).Sub(value, swapFee)
	}
	return big.NewInt(0)
}

// ToBits calc
func ToBits(valueStr string, decimals uint8) *big.Int {
	parts := strings.Split(valueStr, ".")
	if len(parts) > 2 {
		return nil
	}

	ipart, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil
	}

	oneToken := cmath.BigPow(10, int64(decimals))
	result := new(big.Int).Mul(ipart, oneToken)

	var dpart *big.Int
	if len(parts) > 1 {
		dpart, ok = new(big.Int).SetString(parts[1], 10)
		if !ok {
			return nil
		}
		dpart.Mul(dpart, oneToken)
		dpart.Div(dpart, cmath.BigPow(10, int64(len(parts[1]))))
		result.Add(result, dpart)
	}

	return result
}

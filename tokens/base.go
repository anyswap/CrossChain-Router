package tokens

import (
	"math/big"
	"strings"

	cmath "github.com/anyswap/CrossChain-Router/v3/common/math"
)

var (
	swapConfigMap = make(map[string]map[string]*SwapConfig) // key is tokenID,toChainID
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

// SetSwapConfigs set swap configs
func SetSwapConfigs(swapCfgs map[string]map[string]*SwapConfig) {
	swapConfigMap = swapCfgs
}

// GetSwapConfig get swap config
func GetSwapConfig(tokenID, toChainID string) *SwapConfig {
	cfgs := swapConfigMap[tokenID]
	if cfgs == nil {
		return nil
	}
	return cfgs[toChainID]
}

// GetBigValueThreshold get big value threshold
func GetBigValueThreshold(tokenID, toChainID string) *big.Int {
	swapCfg := GetSwapConfig(tokenID, toChainID)
	if swapCfg == nil {
		return big.NewInt(0)
	}
	return swapCfg.BigValueThreshold
}

// CheckTokenSwapValue check swap value is in right range
func CheckTokenSwapValue(tokenID, toChainID string, value *big.Int, fromDecimals, toDecimals uint8) bool {
	if value == nil || value.Sign() <= 0 {
		return false
	}
	swapCfg := GetSwapConfig(tokenID, toChainID)
	if swapCfg == nil {
		return false
	}
	if value.Cmp(swapCfg.MinimumSwap) < 0 {
		return false
	}
	if value.Cmp(swapCfg.MaximumSwap) > 0 {
		return false
	}
	return CalcSwapValue(tokenID, toChainID, value, fromDecimals, toDecimals).Sign() > 0
}

// CalcSwapValue calc swap value (get rid of fee and convert by decimals)
func CalcSwapValue(tokenID, toChainID string, value *big.Int, fromDecimals, toDecimals uint8) *big.Int {
	swapCfg := GetSwapConfig(tokenID, toChainID)
	if swapCfg == nil {
		return big.NewInt(0)
	}
	srcValue := calcSrcSwapValue(swapCfg, value)
	if fromDecimals == toDecimals {
		return srcValue
	}
	// do value convert by decimals
	oneSrcToken := cmath.BigPow(10, int64(fromDecimals))
	oneDstToken := cmath.BigPow(10, int64(toDecimals))
	dstValue := new(big.Int).Mul(srcValue, oneDstToken)
	dstValue.Div(dstValue, oneSrcToken)
	return dstValue
}

func calcSrcSwapValue(swapCfg *SwapConfig, value *big.Int) *big.Int {
	if swapCfg.SwapFeeRatePerMillion == 0 {
		return value
	}

	swapFee := new(big.Int).Mul(value, new(big.Int).SetUint64(swapCfg.SwapFeeRatePerMillion))
	swapFee.Div(swapFee, big.NewInt(1000000))

	if swapFee.Cmp(swapCfg.MinimumSwapFee) < 0 {
		swapFee = swapCfg.MinimumSwapFee
	} else if swapFee.Cmp(swapCfg.MaximumSwapFee) > 0 {
		swapFee = swapCfg.MaximumSwapFee
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

package tokens

import (
	"math/big"
	"strings"

	cmath "github.com/anyswap/CrossChain-Router/v3/common/math"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
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
func GetBigValueThreshold(tokenID, toChainID string, fromDecimals uint8) *big.Int {
	swapCfg := GetSwapConfig(tokenID, toChainID)
	if swapCfg == nil {
		return big.NewInt(0)
	}
	return ConvertTokenValue(swapCfg.BigValueThreshold, 18, fromDecimals)
}

// CheckTokenSwapValue check swap value is in right range
func CheckTokenSwapValue(tokenID, toChainID string, value *big.Int, fromDecimals, toDecimals uint8) bool {
	if params.IsNFTRouter() {
		return true
	}
	if value == nil || value.Sign() <= 0 {
		return false
	}
	swapCfg := GetSwapConfig(tokenID, toChainID)
	if swapCfg == nil {
		return false
	}
	minSwapValue := ConvertTokenValue(swapCfg.MinimumSwap, 18, fromDecimals)
	if value.Cmp(minSwapValue) < 0 {
		return false
	}
	maxSwapValue := ConvertTokenValue(swapCfg.MaximumSwap, 18, fromDecimals)
	if value.Cmp(maxSwapValue) > 0 {
		return false
	}
	return CalcSwapValue(tokenID, toChainID, value, fromDecimals, toDecimals).Sign() > 0
}

// CalcSwapValue calc swap value (get rid of fee and convert by decimals)
func CalcSwapValue(tokenID, toChainID string, value *big.Int, fromDecimals, toDecimals uint8) *big.Int {
	if params.IsNFTRouter() {
		return value
	}
	swapCfg := GetSwapConfig(tokenID, toChainID)
	if swapCfg == nil {
		return big.NewInt(0)
	}

	valueLeft := value
	if swapCfg.SwapFeeRatePerMillion > 0 {
		swapFee := new(big.Int).Mul(value, new(big.Int).SetUint64(swapCfg.SwapFeeRatePerMillion))
		swapFee.Div(swapFee, big.NewInt(1000000))

		minSwapFee := ConvertTokenValue(swapCfg.MinimumSwapFee, 18, fromDecimals)
		if swapFee.Cmp(minSwapFee) < 0 {
			swapFee = minSwapFee
		} else {
			maxSwapFee := ConvertTokenValue(swapCfg.MaximumSwapFee, 18, fromDecimals)
			if swapFee.Cmp(maxSwapFee) > 0 {
				swapFee = maxSwapFee
			}
		}

		var adjustBaseFee *big.Int
		baseFeePercent := params.GetBaseFeePercent(toChainID)
		if baseFeePercent != 0 && minSwapFee.Sign() > 0 {
			adjustBaseFee = new(big.Int).Set(minSwapFee)
			adjustBaseFee.Mul(adjustBaseFee, big.NewInt(baseFeePercent))
			adjustBaseFee.Div(adjustBaseFee, big.NewInt(100))
			swapFee = new(big.Int).Add(swapFee, adjustBaseFee)
			if swapFee.Sign() < 0 {
				swapFee = big.NewInt(0)
			}
		}

		if value.Cmp(swapFee) <= 0 {
			log.Warn("check swap value failed",
				"value", value, "tokenID", tokenID, "toChainID", toChainID,
				"minSwapFee", minSwapFee, "adjustBaseFee", adjustBaseFee, "swapFee", swapFee)
			return big.NewInt(0)
		}

		valueLeft = new(big.Int).Sub(value, swapFee)
	}

	return ConvertTokenValue(valueLeft, fromDecimals, toDecimals)
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

// ConvertTokenValue convert token value
func ConvertTokenValue(fromValue *big.Int, fromDecimals, toDecimals uint8) *big.Int {
	if fromDecimals == toDecimals || fromValue == nil {
		return fromValue
	}
	if fromDecimals > toDecimals {
		return new(big.Int).Div(fromValue, cmath.BigPow(10, int64(fromDecimals-toDecimals)))
	}
	return new(big.Int).Mul(fromValue, cmath.BigPow(10, int64(toDecimals-fromDecimals)))
}

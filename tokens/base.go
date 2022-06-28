package tokens

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	cmath "github.com/anyswap/CrossChain-Router/v3/common/math"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
)

var (
	routerSwapType SwapType

	swapConfigMap = new(sync.Map) // key is tokenID,fromChainID,toChainID
	feeConfigMap  = new(sync.Map) // key is tokenID,fromChainID,toChainID

	// StubChainIDBase stub chainID base value
	StubChainIDBase = big.NewInt(1000000000000)
)

// IsNativeCoin is native coin
func IsNativeCoin(name string) bool {
	return strings.EqualFold(name, "native")
}

// InitRouterSwapType init router swap type
//nolint:goconst // allow dupl constant string
func InitRouterSwapType(swapTypeStr string) {
	switch strings.ToLower(swapTypeStr) {
	case "erc20swap":
		routerSwapType = ERC20SwapType
	case "nftswap":
		routerSwapType = NFTSwapType
	case "anycallswap":
		routerSwapType = AnyCallSwapType
	case "gasconvert":
		routerSwapType = GasConvertSwapType
	default:
		log.Fatalf("invalid router swap type '%v'", swapTypeStr)
	}
	log.Info("init router swap type success", "swaptype", routerSwapType.String())
}

// GetRouterSwapType get router swap type
func GetRouterSwapType() SwapType {
	return routerSwapType
}

// IsERC20Router is erc20 router
func IsERC20Router() bool {
	return routerSwapType == ERC20SwapType
}

// IsNFTRouter is nft router
func IsNFTRouter() bool {
	return routerSwapType == NFTSwapType
}

// IsAnyCallRouter is anycall router
func IsAnyCallRouter() bool {
	return routerSwapType == AnyCallSwapType
}

// IsGasConvertRouter is anycall router
func IsGasConvertRouter() bool {
	return routerSwapType == GasConvertSwapType
}

// CrossChainBridgeBase base bridge
type CrossChainBridgeBase struct {
	ChainConfig    *ChainConfig
	GatewayConfig  *GatewayConfig
	TokenConfigMap *sync.Map // key is token address
	UseFastMPC     bool
}

// NewCrossChainBridgeBase new base bridge
func NewCrossChainBridgeBase() *CrossChainBridgeBase {
	return &CrossChainBridgeBase{
		TokenConfigMap: new(sync.Map),
	}
}

// InitRouterInfo init router info
func (b *CrossChainBridgeBase) InitRouterInfo(routerContract string) error {
	return ErrNotImplemented
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *CrossChainBridgeBase) InitAfterConfig() {
}

// GetBalance get balance is used for checking budgets to prevent DOS attacking
func (b *CrossChainBridgeBase) GetBalance(account string) (*big.Int, error) {
	return nil, ErrNotImplemented
}

// SetChainConfig set chain config
func (b *CrossChainBridgeBase) SetChainConfig(chainCfg *ChainConfig) {
	b.ChainConfig = chainCfg
	if params.IsUseFastMPC(chainCfg.ChainID) {
		b.UseFastMPC = true
	}
}

// SetGatewayConfig set gateway config
func (b *CrossChainBridgeBase) SetGatewayConfig(gatewayCfg *GatewayConfig) {
	if len(gatewayCfg.APIAddress) == 0 {
		log.Fatal("empty gateway 'APIAddress'")
	}
	b.GatewayConfig = gatewayCfg
}

// SetTokenConfig set token config
func (b *CrossChainBridgeBase) SetTokenConfig(token string, tokenCfg *TokenConfig) {
	key := strings.ToLower(token)
	if tokenCfg != nil {
		b.TokenConfigMap.Store(key, tokenCfg)
	} else {
		b.TokenConfigMap.Delete(key)
	}
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
	if config, exist := b.TokenConfigMap.Load(strings.ToLower(token)); exist {
		return config.(*TokenConfig)
	}
	return nil
}

// GetRouterContract get router contract
func (b *CrossChainBridgeBase) GetRouterContract(token string) string {
	if token != "" {
		tokenCfg := b.GetTokenConfig(token)
		if tokenCfg == nil {
			return ""
		}
		if tokenCfg.RouterContract != "" {
			return tokenCfg.RouterContract
		}
	}
	return b.ChainConfig.RouterContract
}

// SetSwapConfigs set swap configs
func SetSwapConfigs(swapCfgs *sync.Map) {
	swapConfigMap = swapCfgs
}

// GetSwapConfig get swap config
func GetSwapConfig(tokenID, fromChainID, toChainID string) *SwapConfig {
	m, exist := swapConfigMap.Load(tokenID)
	if !exist {
		return nil
	}
	mm, exist := m.(*sync.Map).Load(fromChainID)
	if !exist {
		return nil
	}
	mmm, exist := mm.(*sync.Map).Load(toChainID)
	if !exist {
		return nil
	}
	return mmm.(*SwapConfig)
}

// SetFeeConfigs set fee configs
func SetFeeConfigs(feeCfgs *sync.Map) {
	feeConfigMap = feeCfgs
}

// GetFeeConfig get fee config
func GetFeeConfig(tokenID, fromChainID, toChainID string) *FeeConfig {
	m, exist := feeConfigMap.Load(tokenID)
	if !exist {
		return nil
	}
	mm, exist := m.(*sync.Map).Load(fromChainID)
	if !exist {
		return nil
	}
	mmm, exist := mm.(*sync.Map).Load(toChainID)
	if !exist {
		return nil
	}
	return mmm.(*FeeConfig)
}

// GetBigValueThreshold get big value threshold
func GetBigValueThreshold(tokenID, fromChainID, toChainID string, fromDecimals uint8) *big.Int {
	swapCfg := GetSwapConfig(tokenID, fromChainID, toChainID)
	if swapCfg == nil {
		return big.NewInt(0)
	}
	return ConvertTokenValue(swapCfg.BigValueThreshold, 18, fromDecimals)
}

// CheckTokenSwapValue check swap value is in right range
func CheckTokenSwapValue(swapInfo *SwapTxInfo, fromDecimals, toDecimals uint8) bool {
	if !IsGasConvertRouter() {
		return true
	}
	value := swapInfo.Value
	if value == nil || value.Sign() <= 0 {
		return false
	}
	tokenID := swapInfo.GetTokenID()
	fromChainID := swapInfo.FromChainID.String()
	toChainID := swapInfo.ToChainID.String()
	swapCfg := GetSwapConfig(tokenID, fromChainID, toChainID)
	if swapCfg == nil {
		return false
	}
	minSwapValue := ConvertTokenValue(swapCfg.MinimumSwap, 18, fromDecimals)
	if value.Cmp(minSwapValue) < 0 {
		return false
	}
	maxSwapValue := ConvertTokenValue(swapCfg.MaximumSwap, 18, fromDecimals)
	if value.Cmp(maxSwapValue) > 0 &&
		!params.IsInBigValueWhitelist(tokenID, swapInfo.From) &&
		!params.IsInBigValueWhitelist(tokenID, swapInfo.TxTo) {
		return false
	}
	return CalcSwapValue(tokenID, fromChainID, toChainID, value, fromDecimals, toDecimals, swapInfo.From, swapInfo.TxTo).Sign() > 0
}

// CalcSwapValue calc swap value (get rid of fee and convert by decimals)
func CalcSwapValue(tokenID, fromChainID, toChainID string, value *big.Int, fromDecimals, toDecimals uint8, originFrom, originTxTo string) *big.Int {
	if !IsERC20Router() {
		return value
	}
	feeCfg := GetFeeConfig(tokenID, fromChainID, toChainID)
	if feeCfg == nil {
		return big.NewInt(0)
	}

	valueLeft := value
	if feeCfg.SwapFeeRatePerMillion > 0 {
		var swapFee, adjustBaseFee *big.Int
		minSwapFee := ConvertTokenValue(feeCfg.MinimumSwapFee, 18, fromDecimals)
		if params.IsInBigValueWhitelist(tokenID, originFrom) ||
			params.IsInBigValueWhitelist(tokenID, originTxTo) {
			swapFee = minSwapFee
		} else {
			swapFee = new(big.Int).Mul(value, new(big.Int).SetUint64(feeCfg.SwapFeeRatePerMillion))
			swapFee.Div(swapFee, big.NewInt(1000000))

			if swapFee.Cmp(minSwapFee) < 0 {
				swapFee = minSwapFee
			} else {
				maxSwapFee := ConvertTokenValue(feeCfg.MaximumSwapFee, 18, fromDecimals)
				if swapFee.Cmp(maxSwapFee) > 0 {
					swapFee = maxSwapFee
				}
			}

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

// CheckNativeBalance check native balance
func CheckNativeBalance(b IBridge, account string, needValue *big.Int) (err error) {
	var balance *big.Int
	for i := 0; i < 3; i++ {
		balance, err = b.GetBalance(account)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err == nil && balance.Cmp(needValue) < 0 {
		return fmt.Errorf("not enough coin balance. %v is lower than %v needed", balance, needValue)
	}
	if err != nil {
		log.Warn("get balance error", "account", account, "err", err)
	}
	return err
}

package tokens

import (
	"math/big"
	"strings"
	"sync"

	cmath "github.com/anyswap/CrossChain-Router/v3/common/math"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

var (
	routerSwapType SwapType

	swapConfigMap    = new(sync.Map) // key is tokenID,fromChainID,toChainID
	feeConfigMap     = new(sync.Map) // key is tokenID,fromChainID,toChainID
	onchainCustomCfg = new(sync.Map) // key is fromChainID,tokenID

	AggregateIdentifier = "aggregate"

	// StubChainIDBase stub chainID base value
	StubChainIDBase = big.NewInt(1000000000000)
)

// OnchainCustomConfig onchain custom config (in router config)
type OnchainCustomConfig struct {
	AdditionalSrcChainSwapFeeRate uint64
	AdditionalSrcMinimumSwapFee   *big.Int
	AdditionalSrcMaximumSwapFee   *big.Int
}

// IsNativeCoin is native coin
func IsNativeCoin(name string) bool {
	return strings.EqualFold(name, "native")
}

// InitRouterSwapType init router swap type
//
//nolint:goconst // allow dupl constant string
func InitRouterSwapType(swapTypeStr string) {
	switch strings.ToLower(swapTypeStr) {
	case "erc20swap":
		routerSwapType = ERC20SwapType
	case "nftswap":
		routerSwapType = NFTSwapType
	case "anycallswap":
		routerSwapType = AnyCallSwapType
		if !IsValidAnycallSubType(params.GetSwapSubType()) {
			log.Fatalf("invalid anycall sub type '%v'", params.GetSwapSubType())
		}
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

// CrossChainBridgeBase base bridge
type CrossChainBridgeBase struct {
	ChainConfig    *ChainConfig
	GatewayConfig  *GatewayConfig
	TokenConfigMap *sync.Map // key is token address

	RPCClientTimeout int

	UseFastMPC                bool
	DontCheckAddressMixedCase bool
}

// NewCrossChainBridgeBase new base bridge
func NewCrossChainBridgeBase() *CrossChainBridgeBase {
	return &CrossChainBridgeBase{
		TokenConfigMap:   new(sync.Map),
		RPCClientTimeout: client.GetDefaultTimeout(false),
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *CrossChainBridgeBase) InitAfterConfig() {
	chainID := b.ChainConfig.ChainID
	clientTimeout := params.GetRPCClientTimeout(chainID)
	if clientTimeout != 0 {
		b.RPCClientTimeout = clientTimeout
	}
	lclCfg := params.GetLocalChainConfig(chainID)
	b.DontCheckAddressMixedCase = lclCfg.DontCheckAddressMixedCase
}

// InitRouterInfo init router info
func (b *CrossChainBridgeBase) InitRouterInfo(routerContract, routerVersion string) (err error) {
	return ErrNotImplemented
}

// GetBalance get balance is used for checking budgets to prevent DOS attacking
func (b *CrossChainBridgeBase) GetBalance(account string) (*big.Int, error) {
	return nil, ErrNotImplemented
}

// SetChainConfig set chain config
func (b *CrossChainBridgeBase) SetChainConfig(chainCfg *ChainConfig) {
	b.ChainConfig = chainCfg
	if params.IsUseFastMPC(chainCfg.ChainID) {
		log.Info("set chain config use fast mpc", "chainID", chainCfg.ChainID, "chain", chainCfg.BlockChain)
		b.UseFastMPC = true
	}
}

// SetGatewayConfig set gateway config
func (b *CrossChainBridgeBase) SetGatewayConfig(gatewayCfg *GatewayConfig) {
	if gatewayCfg != nil {
		allURLs := gatewayCfg.APIAddress
		if allURLs == nil {
			allURLs = make([]string, 0)
		}
		allURLs = append(allURLs, gatewayCfg.APIAddressExt...)
		existURLs := make(map[string]struct{})
		// get rid of duplicate urls
		for _, url := range allURLs {
			if _, exist := existURLs[url]; exist {
				continue
			}
			existURLs[url] = struct{}{}
			gatewayCfg.AllGatewayURLs = append(gatewayCfg.AllGatewayURLs, url)
		}
		log.Debugf("AllGatewayURLs are %v", gatewayCfg.AllGatewayURLs)
		gatewayCfg.OriginAllGatewayURLs = make([]string, len(gatewayCfg.AllGatewayURLs))
		copy(gatewayCfg.OriginAllGatewayURLs, gatewayCfg.AllGatewayURLs)
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

// GetRouterVersion get router version
func (b *CrossChainBridgeBase) GetRouterVersion(token string) string {
	if token != "" {
		tokenCfg := b.GetTokenConfig(token)
		if tokenCfg == nil {
			return ""
		}
		if tokenCfg.RouterVersion != "" {
			return tokenCfg.RouterVersion
		}
	}
	return b.ChainConfig.RouterVersion
}

// CalcProofID calc proofID
func (b *CrossChainBridgeBase) CalcProofID(args *BuildTxArgs) (string, error) {
	return "", ErrNotImplemented
}

// GenerateProof generate proof
func (b *CrossChainBridgeBase) GenerateProof(proofID string, args *BuildTxArgs) (string, error) {
	return "", ErrNotImplemented
}

// SubmitProof submit proof
func (b *CrossChainBridgeBase) SubmitProof(proofID, proof string, args *BuildTxArgs) (interface{}, string, error) {
	return nil, "", ErrNotImplemented
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

// SetOnchainCustomConfig set onchain custom config
func SetOnchainCustomConfig(chainID, tokenID string, config *OnchainCustomConfig) {
	m, exist := onchainCustomCfg.Load(chainID)
	if exist {
		mm := m.(*sync.Map)
		mm.Store(tokenID, config)
	} else {
		mm := new(sync.Map)
		mm.Store(tokenID, config)
		onchainCustomCfg.Store(chainID, mm)
	}
	cfg := GetOnchainCustomConfig(chainID, tokenID)
	if cfg != config {
		log.Error("set onchain custom config failed", "chainID", chainID, "tokenID", tokenID, "set", config, "get", cfg)
	} else {
		log.Info("set onchain custom config success", "chainID", chainID, "tokenID", tokenID, "config", config)
	}
}

// GetOnchainCustomConfig get onchain custom config
func GetOnchainCustomConfig(chainID, tokenID string) *OnchainCustomConfig {
	if m, exist := onchainCustomCfg.Load(chainID); exist {
		mm := m.(*sync.Map)
		if c, ok := mm.Load(tokenID); ok {
			return c.(*OnchainCustomConfig)
		}
	}
	return nil
}

// GetBigValueThreshold get big value threshold
func GetBigValueThreshold(tokenID, fromChainID, toChainID string, fromDecimals uint8) *big.Int {
	swapCfg := GetSwapConfig(tokenID, fromChainID, toChainID)
	if swapCfg == nil {
		return big.NewInt(0)
	}
	value := ConvertTokenValue(swapCfg.BigValueThreshold, 18, fromDecimals)
	discount := params.GetLocalChainConfig(fromChainID).BigValueDiscount
	if discount > 0 && discount < 100 {
		value.Mul(value, new(big.Int).SetUint64(discount))
		value.Div(value, big.NewInt(100))
	}
	return value
}

// CheckTokenSwapValue check swap value is in right range
func CheckTokenSwapValue(swapInfo *SwapTxInfo, fromDecimals, toDecimals uint8) bool {
	if !IsERC20Router() {
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

	swapfeeRatePerMillion := feeCfg.SwapFeeRatePerMillion
	minimumSwapFee := feeCfg.MinimumSwapFee
	maximumSwapFee := feeCfg.MaximumSwapFee

	useFixedFee := minimumSwapFee.Sign() > 0 && minimumSwapFee.Cmp(maximumSwapFee) == 0
	if useFixedFee {
		swapfeeRatePerMillion = 0
	}

	srcFeeCfg := GetOnchainCustomConfig(fromChainID, tokenID)

	var srcFeeRate uint64
	if srcFeeCfg != nil {
		srcFeeRate = srcFeeCfg.AdditionalSrcChainSwapFeeRate
		swapfeeRatePerMillion += srcFeeRate
		minimumSwapFee = cmath.BigMax(feeCfg.MinimumSwapFee, srcFeeCfg.AdditionalSrcMinimumSwapFee)
		maximumSwapFee = cmath.BigMax(feeCfg.MaximumSwapFee, srcFeeCfg.AdditionalSrcMaximumSwapFee)
	}

	valueLeft := value
	if swapfeeRatePerMillion > 0 || useFixedFee {
		log.Info("calc swap fee start",
			"tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID,
			"value", value, "feeRate", swapfeeRatePerMillion, "useFixedFee", useFixedFee,
			"cfgFeeRate", feeCfg.SwapFeeRatePerMillion, "srcFeeRate", srcFeeRate,
			"minFee", minimumSwapFee, "maxFee", maximumSwapFee)

		var swapFee, adjustBaseFee *big.Int
		minSwapFee := ConvertTokenValue(minimumSwapFee, 18, fromDecimals)
		if params.IsInBigValueWhitelist(tokenID, originFrom) ||
			params.IsInBigValueWhitelist(tokenID, originTxTo) {
			swapFee = minSwapFee
		} else {
			if swapfeeRatePerMillion > 0 {
				swapFee = new(big.Int).Mul(value, new(big.Int).SetUint64(swapfeeRatePerMillion))
				swapFee.Div(swapFee, big.NewInt(1000000))
			} else {
				swapFee = big.NewInt(0)
			}

			if useFixedFee {
				fixedFee := ConvertTokenValue(feeCfg.MinimumSwapFee, 18, fromDecimals)
				swapFee = cmath.BigMax(swapFee, fixedFee)
			}

			if swapFee.Cmp(minSwapFee) < 0 {
				swapFee = minSwapFee
			} else {
				maxSwapFee := ConvertTokenValue(maximumSwapFee, 18, fromDecimals)
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
				"tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID,
				"value", value, "swapFee", swapFee, "adjustBaseFee", adjustBaseFee)
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

package tokens

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	cmath "github.com/anyswap/CrossChain-Router/v3/common/math"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	ethclient "github.com/jowenshaw/gethclient"
	ethcommon "github.com/jowenshaw/gethclient/common"
	"github.com/jowenshaw/gethclient/types/ethereum"
)

var (
	routerSwapType SwapType

	swapConfigMap = new(sync.Map) // key is tokenID,fromChainID,toChainID
	feeConfigMap  = new(sync.Map) // key is tokenID,fromChainID,toChainID

	// StubChainIDBase stub chainID base value
	StubChainIDBase = big.NewInt(1000000000000)
	routerConfigCtx = context.Background()
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
	case "gasswap":
		routerSwapType = GasSwapType
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

// IsGasSwapRouter is anycall router
func IsGasSwapRouter() bool {
	return routerSwapType == GasSwapType
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

func CallPriceOracleContract(data hexutil.Bytes, blockNumber string) (result []byte, err error) {
	priceOracleConfig := params.GetRouterConfig().PriceOracle
	contract := ethcommon.HexToAddress(priceOracleConfig.Contract)
	msg := ethereum.CallMsg{
		To:   &contract,
		Data: data,
	}
	for _, url := range priceOracleConfig.APIAddress {
		cli, err := ethclient.Dial(url)
		if err != nil {
			log.Fatal("init price oracle web socket clients failed", "url", url, "err", err)
		}
		result, err = cli.CallContract(routerConfigCtx, msg, nil)
		if err == nil {
			return result, nil
		}
	}
	log.Debug("call price contract error", "contract", contract.String(), "data", data, "err", err)
	return nil, ErrCallPriceOracle
}

func GetCurrencyInfo(chainID *big.Int) (*CurrencyInfo, error) {
	funcHash := common.FromHex("0x3cc7f0fa")
	data := abicoder.PackDataWithFuncHash(funcHash, chainID)
	res, err := CallPriceOracleContract(data, "latest")
	if err != nil {
		return nil, err
	}
	price := common.GetBigInt(res, 0, 32)
	decimal := common.GetBigInt(res, 32, 32)
	if price.Cmp(big.NewInt(0)) == 0 || decimal.Cmp(big.NewInt(0)) == 0 {
		return nil, ErrOraclePrice
	}
	return &CurrencyInfo{
		Price:   price,
		Decimal: decimal,
	}, nil
}

func GetSwapThreshold(chainID *big.Int) (*big.Int, error) {
	funcHash := common.FromHex("0xd3857520")
	data := abicoder.PackDataWithFuncHash(funcHash, chainID)
	res, err := CallPriceOracleContract(data, "latest")
	if err != nil {
		return nil, err
	}
	high := common.GetBigInt(res, 64, 32)
	return high, nil
}

func CheckGasSwapValue(fromChainID *big.Int, gasSwapInfo *GasSwapInfo, receiveValue *big.Int) (*big.Int, error) {
	srcCurrencyPrice := gasSwapInfo.SrcCurrencyPrice
	destCurrencyPrice := gasSwapInfo.DestCurrencyPrice
	srcDecimal := uint8(gasSwapInfo.SrcCurrencyDecimal.Uint64())
	destDecimal := uint8(gasSwapInfo.DestCurrencyDecimal.Uint64())
	minReceiveValue := gasSwapInfo.MinReceiveValue

	swapInTotalPrice := new(big.Int).Mul(srcCurrencyPrice, receiveValue)
	swapInTotalPrice = ConvertTokenValue(swapInTotalPrice, srcDecimal, 0)

	swapThreshold, err := GetSwapThreshold(fromChainID)
	if err != nil {
		return nil, err
	}

	if swapThreshold.Cmp(swapInTotalPrice) < 0 {
		log.Error("CheckGasSwapValue", "swapThreshold", swapThreshold, "swapInTotalPrice", swapInTotalPrice)
		return nil, ErrOutOfSwapThreshold
	}

	receiveTotalValue := srcCurrencyPrice.Mul(srcCurrencyPrice, receiveValue)
	amount := receiveTotalValue.Div(receiveTotalValue, destCurrencyPrice)

	realReceiveValue := ConvertTokenValue(amount, srcDecimal, destDecimal)

	if realReceiveValue.Cmp(minReceiveValue) < 0 {
		log.Error("CheckGasSwapValue", "minReceiveValue", minReceiveValue, "realReceiveValue", realReceiveValue)
		return nil, ErrLessThanMinValue
	}

	upperThreshold := new(big.Int).Mul(minReceiveValue, big.NewInt(120))
	upperThreshold = upperThreshold.Div(upperThreshold, big.NewInt(100))

	if upperThreshold.Cmp(realReceiveValue) < 0 {
		realReceiveValue = upperThreshold
	}

	log.Warn("buildGasSwapTxInput", "srcPrice", gasSwapInfo.SrcCurrencyPrice, "destPrice", gasSwapInfo.DestCurrencyPrice, "minReceiveValue", minReceiveValue, "realReceiveValue", realReceiveValue, "upperThreshold", upperThreshold)
	return realReceiveValue, nil
}

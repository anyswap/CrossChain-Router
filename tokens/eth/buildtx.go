package eth

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second

	cachedNonce = make(map[string]uint64)
)

// BuildRawTransaction build raw tx
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.Input != nil {
		return nil, fmt.Errorf("forbid build raw swap tx with input data")
	}
	if args.From == "" {
		return nil, fmt.Errorf("forbid empty sender")
	}
	routerMPC, err := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}

	switch args.SwapType {
	case tokens.ERC20SwapType:
		err = b.buildERC20SwapTxInput(args)
	case tokens.NFTSwapType:
		err = b.buildNFTSwapTxInput(args)
	case tokens.AnyCallSwapType:
		err = b.buildAnyCallSwapTxInput(args)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	if err != nil {
		return nil, err
	}

	err = b.setDefaults(args)
	if err != nil {
		return nil, err
	}

	return b.buildTx(args)
}

func (b *Bridge) buildTx(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	var (
		to        = common.HexToAddress(args.To)
		value     = args.Value
		input     = *args.Input
		extra     = args.Extra.EthExtra
		gasLimit  = *extra.Gas
		gasPrice  = extra.GasPrice
		gasTipCap = extra.GasTipCap
		gasFeeCap = extra.GasFeeCap

		isDynamicFeeTx = params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID)
	)

	if params.IsSwapServer ||
		(params.GetRouterOracleConfig() != nil &&
			params.GetRouterOracleConfig().CheckGasTokenBalance) {
		minReserveFee := b.getMinReserveFee()
		// if min reserve fee is zero, then do not check balance
		if minReserveFee.Sign() > 0 {
			// swap need value = tx value + min reserve + 5 * gas fee
			needValue := big.NewInt(0)
			if value != nil && value.Sign() > 0 {
				needValue.Add(needValue, value)
			}
			needValue.Add(needValue, minReserveFee)
			var gasFee *big.Int
			if isDynamicFeeTx {
				gasFee = new(big.Int).Mul(gasFeeCap, new(big.Int).SetUint64(gasLimit))
			} else {
				gasFee = new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
			}
			needValue.Add(needValue, new(big.Int).Mul(big.NewInt(5), gasFee))

			err = b.checkCoinBalance(args.From, needValue)
			if err != nil {
				log.Warn("not enough coin balance", "tx.value", value, "gasFee", gasFee, "gasLimit", gasLimit, "gasPrice", gasPrice, "gasFeeCap", gasFeeCap, "minReserveFee", minReserveFee, "needValue", needValue, "isDynamic", isDynamicFeeTx, "swapID", args.SwapID, "err", err)
				return nil, err
			}
		}
	}

	// assign nonce immediately before construct tx
	// esp. for parallel signing, this can prevent nonce hole
	if extra.Nonce == nil { // server logic
		extra.Nonce, err = b.getAccountNonce(args)
		if err != nil {
			return nil, err
		}
	}

	nonce := *extra.Nonce

	key := strings.ToLower(fmt.Sprintf("%v:%v", b.ChainConfig.ChainID, args.From))
	cached := cachedNonce[key]
	if (cached > 0 && (nonce > cached+1000 || nonce+1000 < cached)) ||
		(cached == 0 && nonce > 10000000) {
		return nil, fmt.Errorf("nonce is out of range. cached %v, your %v", cached, nonce)
	}
	cachedNonce[key] = nonce

	if isDynamicFeeTx {
		rawTx = types.NewDynamicFeeTx(b.SignerChainID, nonce, &to, value, gasLimit, gasTipCap, gasFeeCap, input, nil)
	} else {
		rawTx = types.NewTransaction(nonce, to, value, gasLimit, gasPrice, input)
	}

	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "to", to.String(), "bind", args.Bind, "nonce", nonce,
		"gasLimit", gasLimit, "replaceNum", args.GetReplaceNum(),
	}
	if gasTipCap != nil || gasFeeCap != nil {
		ctx = append(ctx, "gasTipCap", gasTipCap, "gasFeeCap", gasFeeCap)
	} else {
		ctx = append(ctx, "gasPrice", gasPrice)
	}
	switch {
	case args.ERC20SwapInfo != nil:
		ctx = append(ctx,
			"originValue", args.OriginValue,
			"swapValue", args.SwapValue,
			"tokenID", args.ERC20SwapInfo.TokenID)
	case args.NFTSwapInfo != nil:
		ctx = append(ctx,
			"tokenID", args.NFTSwapInfo.TokenID,
			"ids", args.NFTSwapInfo.IDs,
			"amounts", args.NFTSwapInfo.Amounts,
			"batch", args.NFTSwapInfo.Batch)
	}
	log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()), ctx...)

	return rawTx, nil
}

func getOrInitEthExtra(args *tokens.BuildTxArgs) *tokens.EthExtraArgs {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{EthExtra: &tokens.EthExtraArgs{}}
	} else if args.Extra.EthExtra == nil {
		args.Extra.EthExtra = &tokens.EthExtraArgs{}
	}
	return args.Extra.EthExtra
}

func (b *Bridge) setDefaults(args *tokens.BuildTxArgs) (err error) {
	if args.Value == nil {
		args.Value = new(big.Int)
	}
	extra := getOrInitEthExtra(args)
	if params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID) {
		if extra.GasTipCap == nil {
			extra.GasTipCap, err = b.getGasTipCap(args)
			if err != nil {
				return err
			}
		}
		if extra.GasFeeCap == nil {
			extra.GasFeeCap, err = b.getGasFeeCap(args, extra.GasTipCap)
			if err != nil {
				return err
			}
		}
		extra.GasPrice = nil
	} else if extra.GasPrice == nil {
		extra.GasPrice, err = b.getGasPrice(args)
		if err != nil {
			return err
		}
		extra.GasTipCap = nil
		extra.GasFeeCap = nil
	}
	if extra.Gas == nil {
		esGasLimit, errf := b.EstimateGas(args.From, args.To, args.Value, *args.Input)
		if errf != nil {
			log.Error(fmt.Sprintf("build %s tx estimate gas failed", args.SwapType.String()),
				"swapID", args.SwapID, "from", args.From, "to", args.To,
				"value", args.Value, "data", *args.Input, "err", errf)
			return fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, tokens.ErrEstimateGasFailed)
		}
		// eg. ranger chain call eth_estimateGas return no error and 0 value in failure situation
		if esGasLimit == 0 && params.GetLocalChainConfig(b.ChainConfig.ChainID).EstimatedGasMustBePositive {
			log.Error(fmt.Sprintf("build %s tx estimate gas return 0", args.SwapType.String()),
				"swapID", args.SwapID, "from", args.From, "to", args.To,
				"value", args.Value, "data", *args.Input)
			return fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, tokens.ErrEstimateGasFailed)
		}
		esGasLimit += esGasLimit * 30 / 100
		defGasLimit := b.getDefaultGasLimit()
		if esGasLimit < defGasLimit {
			esGasLimit = defGasLimit
		}
		// max token gas limit consider first, then max chain gas limit
		maxTokenGasLimit := params.GetMaxTokenGasLimit(args.GetTokenID(), b.ChainConfig.ChainID)
		if maxTokenGasLimit > 0 {
			if esGasLimit > maxTokenGasLimit {
				log.Warn(fmt.Sprintf("build %s tx estimated gas is too large", args.SwapType.String()),
					"swapID", args.SwapID, "from", args.From, "to", args.To,
					"value", args.Value, "data", *args.Input,
					"gasLimit", esGasLimit, "maxTokenGasLimit", maxTokenGasLimit)
				return fmt.Errorf("%w %v %v on chain %v", tokens.ErrBuildTxErrorAndDelay, "estimated gas is too large for token", args.GetTokenID(), b.ChainConfig.ChainID)
			}
		} else {
			maxGasLimit := params.GetMaxGasLimit(b.ChainConfig.ChainID)
			if maxGasLimit > 0 && esGasLimit > maxGasLimit {
				log.Warn(fmt.Sprintf("build %s tx estimated gas is too large", args.SwapType.String()),
					"swapID", args.SwapID, "from", args.From, "to", args.To,
					"value", args.Value, "data", *args.Input,
					"gasLimit", esGasLimit, "maxGasLimit", maxGasLimit)
				return fmt.Errorf("%w %v on chain %v", tokens.ErrBuildTxErrorAndDelay, "estimated gas is too large", b.ChainConfig.ChainID)
			}
		}
		extra.Gas = new(uint64)
		*extra.Gas = esGasLimit
	}
	return nil
}

func (b *Bridge) getDefaultGasLimit() uint64 {
	gasLimit := uint64(90000)
	serverCfg := params.GetRouterServerConfig()
	if serverCfg != nil {
		if cfgGasLimit, exist := serverCfg.DefaultGasLimit[b.ChainConfig.ChainID]; exist {
			gasLimit = cfgGasLimit
		}
	}
	return gasLimit
}

func (b *Bridge) getGasPrice(args *tokens.BuildTxArgs) (price *big.Int, err error) {
	fixedGasPrice := params.GetFixedGasPrice(b.ChainConfig.ChainID)
	if fixedGasPrice != nil {
		if params.IsDebugMode() { // debug the get gas price function
			if price1, err1 := b.SuggestPrice(); err1 == nil {
				max := params.GetMaxGasPrice(b.ChainConfig.ChainID)
				if max != nil && price1.Cmp(max) > 0 {
					log.Warnf("call eth_gasPrice got gas price %v exceeded maximum limit %v", price1, max)
				}
			}
		}
		price = fixedGasPrice
		if args.GetReplaceNum() == 0 {
			return price, nil
		}
		maxGasPrice := params.GetMaxGasPrice(b.ChainConfig.ChainID)
		if maxGasPrice != nil && price.Cmp(maxGasPrice) == 0 {
			return price, nil
		}
	} else {
		for i := 0; i < retryRPCCount; i++ {
			price, err = b.SuggestPrice()
			if err == nil {
				break
			}
			time.Sleep(retryRPCInterval)
		}
		if err != nil {
			return nil, err
		}
	}

	if params.IsTestMode {
		return price, nil
	}

	price, err = b.adjustSwapGasPrice(args, price)
	if err != nil {
		return nil, err
	}

	maxGasPrice := params.GetMaxGasPrice(b.ChainConfig.ChainID)
	if maxGasPrice != nil && price.Cmp(maxGasPrice) > 0 {
		log.Warn("gas price exceeded maximum limit", "chainID", b.ChainConfig.ChainID, "gasPrice", price, "max", maxGasPrice)
		return nil, fmt.Errorf("gas price %v exceeded config maximum limit", price)
	}
	if maxGasPrice == nil && price.Cmp(b.autoMaxGasPrice) > 0 {
		log.Warn("gas price exceeded auto maximum limit", "chainID", b.ChainConfig.ChainID, "gasPrice", price, "autoMax", b.autoMaxGasPrice)
		return nil, fmt.Errorf("gas price %v exceeded auto maximum limit", price)
	}

	smallestGasPriceUnit := params.GetLocalChainConfig(b.ChainConfig.ChainID).SmallestGasPriceUnit
	if smallestGasPriceUnit > 0 {
		smallestUnit := new(big.Int).SetUint64(smallestGasPriceUnit)
		remainder := new(big.Int).Mod(price, smallestUnit)
		if remainder.Sign() != 0 {
			price = new(big.Int).Add(price, smallestUnit)
			price = new(big.Int).Sub(price, remainder)
		}
	}

	return price, nil
}

// args and oldGasPrice should be read only
func (b *Bridge) adjustSwapGasPrice(args *tokens.BuildTxArgs, oldGasPrice *big.Int) (newGasPrice *big.Int, err error) {
	serverCfg := params.GetRouterServerConfig()
	if serverCfg == nil {
		return nil, fmt.Errorf("no router server config")
	}
	newGasPrice = new(big.Int).Set(oldGasPrice) // clone from old
	addPercent := uint64(0)
	if !params.IsFixedGasPrice(b.ChainConfig.ChainID) {
		addPercent = serverCfg.PlusGasPricePercentage
	}
	replaceNum := args.GetReplaceNum()
	if replaceNum > 0 {
		addPercent += replaceNum * serverCfg.ReplacePlusGasPricePercent
	}
	if addPercent > serverCfg.MaxPlusGasPricePercentage {
		addPercent = serverCfg.MaxPlusGasPricePercentage
	}
	if addPercent > 0 {
		newGasPrice.Mul(newGasPrice, big.NewInt(int64(100+addPercent)))
		newGasPrice.Div(newGasPrice, big.NewInt(100))
	}
	maxGasPriceFluctPercent := serverCfg.MaxGasPriceFluctPercent
	if maxGasPriceFluctPercent > 0 {
		if b.latestGasPrice != nil {
			maxFluct := new(big.Int).Set(b.latestGasPrice)
			maxFluct.Mul(maxFluct, new(big.Int).SetUint64(maxGasPriceFluctPercent))
			maxFluct.Div(maxFluct, big.NewInt(100))
			minGasPrice := new(big.Int).Sub(b.latestGasPrice, maxFluct)
			if newGasPrice.Cmp(minGasPrice) < 0 {
				newGasPrice = minGasPrice
			}
		}
		if replaceNum == 0 { // exclude replace situation
			b.latestGasPrice = newGasPrice
		}
	}
	tempMaxGasPrice := new(big.Int).Mul(newGasPrice, big.NewInt(10))
	if b.autoMaxGasPrice == nil || b.autoMaxGasPrice.Cmp(tempMaxGasPrice) > 0 {
		b.autoMaxGasPrice = tempMaxGasPrice
	} else {
		added := new(big.Int).Div(b.autoMaxGasPrice, big.NewInt(10))
		b.autoMaxGasPrice = new(big.Int).Add(b.autoMaxGasPrice, added)
	}
	return newGasPrice, nil
}

func (b *Bridge) getAccountNonce(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

	res, err := b.getPoolNonce(args)
	if err != nil {
		return nil, err
	}

	nonce = b.AdjustNonce(args.From, *res)
	return &nonce, nil
}

func (b *Bridge) getPoolNonce(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	getPoolNonceBlockNumberOpt := "pending" // latest or pending
	if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) {
		getPoolNonceBlockNumberOpt = "latest"
	}

	for i := 0; i < retryRPCCount; i++ {
		nonce, err = b.GetPoolNonce(args.From, getPoolNonceBlockNumberOpt)
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err != nil {
		return nil, err
	}

	return &nonce, nil
}

func (b *Bridge) getMinReserveFee() *big.Int {
	config := params.GetRouterConfig()
	if config == nil {
		return big.NewInt(0)
	}
	minReserve := params.GetMinReserveFee(b.ChainConfig.ChainID)
	if minReserve == nil {
		minReserve = big.NewInt(1e17) // default 0.1 ETH
	}
	return minReserve
}

func (b *Bridge) checkCoinBalance(sender string, needValue *big.Int) (err error) {
	var balance *big.Int
	for i := 0; i < retryRPCCount; i++ {
		balance, err = b.GetBalance(sender)
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err == nil && balance.Cmp(needValue) < 0 {
		return fmt.Errorf("not enough coin balance. %v < %v", balance, needValue)
	}
	if err != nil {
		log.Warn("get balance error", "sender", sender, "err", err)
	}
	return err
}

func (b *Bridge) getGasTipCap(args *tokens.BuildTxArgs) (gasTipCap *big.Int, err error) {
	serverCfg := params.GetRouterServerConfig()
	dfConfig := params.GetDynamicFeeTxConfig(b.ChainConfig.ChainID)
	if serverCfg == nil || dfConfig == nil {
		return nil, tokens.ErrMissDynamicFeeConfig
	}

	for i := 0; i < retryRPCCount; i++ {
		gasTipCap, err = b.SuggestGasTipCap()
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err != nil {
		return nil, err
	}

	addPercent := dfConfig.PlusGasTipCapPercent
	replaceNum := args.GetReplaceNum()
	if replaceNum > 0 {
		addPercent += replaceNum * serverCfg.ReplacePlusGasPricePercent
	}
	if addPercent > serverCfg.MaxPlusGasPricePercentage {
		addPercent = serverCfg.MaxPlusGasPricePercentage
	}
	if addPercent > 0 {
		gasTipCap.Mul(gasTipCap, big.NewInt(int64(100+addPercent)))
		gasTipCap.Div(gasTipCap, big.NewInt(100))
	}

	maxGasTipCap := dfConfig.GetMaxGasTipCap()
	if maxGasTipCap != nil && gasTipCap.Cmp(maxGasTipCap) > 0 {
		gasTipCap = maxGasTipCap
	}
	return gasTipCap, nil
}

func (b *Bridge) getGasFeeCap(_ *tokens.BuildTxArgs, gasTipCap *big.Int) (gasFeeCap *big.Int, err error) {
	dfConfig := params.GetDynamicFeeTxConfig(b.ChainConfig.ChainID)
	if dfConfig == nil {
		return nil, tokens.ErrMissDynamicFeeConfig
	}

	blockCount := dfConfig.BlockCountFeeHistory
	var baseFee *big.Int
	for i := 0; i < retryRPCCount; i++ {
		baseFee, err = b.GetBaseFee(blockCount)
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err != nil {
		return nil, err
	}

	newGasFeeCap := new(big.Int).Set(gasTipCap) // copy
	newGasFeeCap.Add(newGasFeeCap, baseFee.Mul(baseFee, big.NewInt(2)))

	newGasFeeCap.Mul(newGasFeeCap, big.NewInt(int64(100+dfConfig.PlusGasFeeCapPercent)))
	newGasFeeCap.Div(newGasFeeCap, big.NewInt(100))

	maxGasFeeCap := dfConfig.GetMaxGasFeeCap()
	if maxGasFeeCap != nil && newGasFeeCap.Cmp(maxGasFeeCap) > 0 {
		newGasFeeCap = maxGasFeeCap
	}
	return newGasFeeCap, nil
}

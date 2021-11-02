package eth

import (
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second

	latestGasPrice *big.Int
)

// BuildRawTransaction build raw tx
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if args.Input != nil {
		return nil, fmt.Errorf("forbid build raw swap tx with input data")
	}
	if args.From == "" {
		return nil, fmt.Errorf("forbid empty sender")
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

	extra, err := b.setDefaults(args)
	if err != nil {
		return nil, err
	}

	return b.buildTx(args, extra)
}

func (b *Bridge) buildTx(args *tokens.BuildTxArgs, extra *tokens.EthExtraArgs) (rawTx interface{}, err error) {
	var (
		to        = common.HexToAddress(args.To)
		value     = args.Value
		input     = *args.Input
		nonce     = *extra.Nonce
		gasLimit  = *extra.Gas
		gasPrice  = extra.GasPrice
		gasTipCap = extra.GasTipCap
		gasFeeCap = extra.GasFeeCap

		isDynamicFeeTx = params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID)
	)

	needValue := b.getMinReserveFee()
	if value != nil {
		needValue.Add(needValue, value)
	}

	err = b.checkCoinBalance(args.From, needValue)
	if err != nil {
		return nil, err
	}

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

func (b *Bridge) setDefaults(args *tokens.BuildTxArgs) (extra *tokens.EthExtraArgs, err error) {
	if args.Value == nil {
		args.Value = new(big.Int)
	}
	extra = getOrInitEthExtra(args)
	if params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID) {
		if extra.GasTipCap == nil {
			extra.GasTipCap, err = b.getGasTipCap(args)
			if err != nil {
				return nil, err
			}
		}
		if extra.GasFeeCap == nil {
			extra.GasFeeCap, err = b.getGasFeeCap(args, extra.GasTipCap)
			if err != nil {
				return nil, err
			}
		}
		extra.GasPrice = nil
	} else if extra.GasPrice == nil {
		extra.GasPrice, err = b.getGasPrice(args)
		if err != nil {
			return nil, err
		}
		extra.GasTipCap = nil
		extra.GasFeeCap = nil
	}
	if extra.Nonce == nil {
		extra.Nonce, err = b.getAccountNonce(args.From)
		if err != nil {
			return nil, err
		}
	}
	if extra.Gas == nil {
		esGasLimit, errf := b.EstimateGas(args.From, args.To, args.Value, *args.Input)
		if errf != nil {
			log.Error(fmt.Sprintf("build %s tx estimate gas failed", args.SwapType.String()),
				"swapID", args.SwapID, "from", args.From, "to", args.To,
				"value", args.Value, "data", *args.Input, "err", errf)
			return nil, tokens.ErrEstimateGasFailed
		}
		esGasLimit += esGasLimit * 30 / 100
		defGasLimit := b.getDefaultGasLimit()
		if esGasLimit < defGasLimit {
			esGasLimit = defGasLimit
		}
		extra.Gas = new(uint64)
		*extra.Gas = esGasLimit
	}
	return extra, nil
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
		return fixedGasPrice, nil
	}

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

	price, err = b.adjustSwapGasPrice(args, price)
	if err != nil {
		return nil, err
	}

	maxGasPrice := params.GetMaxGasPrice(b.ChainConfig.ChainID)
	if maxGasPrice != nil && price.Cmp(maxGasPrice) > 0 {
		return nil, fmt.Errorf("gas price %v exceeded maximum limit", price)
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
	addPercent := serverCfg.PlusGasPricePercentage
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
		if latestGasPrice != nil {
			maxFluct := new(big.Int).Set(latestGasPrice)
			maxFluct.Mul(maxFluct, new(big.Int).SetUint64(maxGasPriceFluctPercent))
			maxFluct.Div(maxFluct, big.NewInt(100))
			minGasPrice := new(big.Int).Sub(latestGasPrice, maxFluct)
			if newGasPrice.Cmp(minGasPrice) < 0 {
				newGasPrice = minGasPrice
			}
		}
		if replaceNum == 0 { // exclude replace situation
			latestGasPrice = newGasPrice
		}
	}
	return newGasPrice, nil
}

func (b *Bridge) getAccountNonce(from string) (nonceptr *uint64, err error) {
	var nonce uint64
	for i := 0; i < retryRPCCount; i++ {
		nonce, err = b.GetPoolNonce(from, "pending")
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err != nil {
		return nil, err
	}
	nonce = b.AdjustNonce(from, nonce)
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

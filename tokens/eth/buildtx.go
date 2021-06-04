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
	case tokens.RouterSwapType:
		err = b.buildRouterSwapTxInput(args)
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
		to       = common.HexToAddress(args.To)
		value    = args.Value
		input    = *args.Input
		nonce    = *extra.Nonce
		gasLimit = *extra.Gas
		gasPrice = extra.GasPrice
	)

	err = b.checkBalance(args.From, value, gasPrice, gasLimit, true)
	if err != nil {
		return nil, err
	}

	rawTx = types.NewTransaction(nonce, to, value, gasLimit, gasPrice, input)

	log.Info("build routerswap raw tx", "swapID", args.SwapID,
		"from", args.From, "to", to.String(), "bind", args.Bind, "nonce", nonce,
		"value", value, "originValue", args.OriginValue, "swapValue", args.SwapValue,
		"gasLimit", gasLimit, "gasPrice", gasPrice, "replaceNum", args.ReplaceNum)

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
	if extra.GasPrice == nil {
		extra.GasPrice, err = b.getGasPrice(args)
		if err != nil {
			return nil, err
		}
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
			log.Error("build routerswap tx estimate gas failed",
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
	if args != nil && args.SwapType != tokens.NonSwapType {
		price, err = b.adjustSwapGasPrice(args, price)
		if err != nil {
			return nil, err
		}
	}
	return price, err
}

// args and oldGasPrice should be read only
func (b *Bridge) adjustSwapGasPrice(args *tokens.BuildTxArgs, oldGasPrice *big.Int) (newGasPrice *big.Int, err error) {
	serverCfg := params.GetRouterServerConfig()
	if serverCfg == nil {
		return nil, fmt.Errorf("no router server config")
	}
	newGasPrice = new(big.Int).Set(oldGasPrice) // clone from old
	addPercent := serverCfg.PlusGasPricePercentage
	if args.ReplaceNum > 0 {
		addPercent += args.ReplaceNum * serverCfg.ReplacePlusGasPricePercent
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
		if args.ReplaceNum == 0 { // exclude replace situation
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

func (b *Bridge) checkBalance(sender string, amount, gasPrice *big.Int, gasLimit uint64, isBuildTx bool) (err error) {
	var needValue *big.Int
	if amount != nil {
		needValue = new(big.Int).Set(amount)
	} else {
		needValue = big.NewInt(0)
	}
	gasFee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
	if isBuildTx {
		gasFee.Mul(gasFee, big.NewInt(3)) // amplify gas gee to keep some reserve
	}
	needValue.Add(needValue, gasFee)

	var balance *big.Int
	for i := 0; i < retryRPCCount; i++ {
		balance, err = b.GetBalance(sender)
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err == nil && balance.Cmp(amount) < 0 {
		return fmt.Errorf("not enough coin balance. %v < %v", balance, needValue)
	}
	if err != nil {
		log.Warn("get balance error", "sender", sender, "err", err)
	}
	return err
}

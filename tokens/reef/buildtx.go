package reef

import (
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"

	substratetypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second

	latestGasPrice *big.Int
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
		storageLimit = uint64(10000)

		isDynamicFeeTx = params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID)
	)

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
			return nil, err
		}
	}

	// assign nonce immediately before construct tx
	// esp. for parallel signing, this can prevent nonce hole
	if extra.Nonce == nil {
		extra.Nonce, err = b.getAccountNonce(args)
		if err != nil {
			return nil, err
		}
	}
	nonce := *extra.Nonce

	var metadata substratetypes.Metadata
	metaraw :=b.GetMetadata()
	err = substratetypes.DecodeFromHex(*metaraw, &metadata)
	if err != nil { return nil, err }
	rv, err := b.GetRuntimeVersionLatest()
	if err != nil { return nil, err }
	genesisHashStr, err := b.GetBlockHashByNumber(big.NewInt(0))
	if err != nil { return nil, err }
	genesisHash, err := substratetypes.NewHashFromHexString(genesisHashStr)
	if err != nil { return nil, err }

	c, err := substratetypes.NewCall(&metadata, "EVM.call", to, input, 0, gasLimit, storageLimit)
	if err != nil { return nil, err }
	
	// Create the extrinsic
	ext := substratetypes.NewExtrinsic(c)
	rawTx = &Extrinsic{
		Extrinsic: &ext,
		SignatureOptions: &substratetypes.SignatureOptions{
			BlockHash:          genesisHash,
			Era:                substratetypes.ExtrinsicEra{IsMortalEra: false},
			GenesisHash:        genesisHash,
			Nonce:              substratetypes.NewUCompactFromUInt(nonce),
			SpecVersion:        rv.SpecVersion,
			Tip:                substratetypes.NewUCompactFromUInt(0),
		},
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

	if extra.Gas == nil {
		esGasLimit, errf := b.EstimateGas(args.From, args.To, args.Value, *args.Input)
		if errf != nil {
			log.Error(fmt.Sprintf("build %s tx estimate gas failed", args.SwapType.String()),
				"swapID", args.SwapID, "from", args.From, "to", args.To,
				"value", args.Value, "data", *args.Input, "err", errf)
			return tokens.ErrEstimateGasFailed
		}
		esGasLimit += esGasLimit * 30 / 100
		defGasLimit := b.getDefaultGasLimit()
		if esGasLimit < defGasLimit {
			esGasLimit = defGasLimit
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
		price = fixedGasPrice
		if args.GetReplaceNum() == 0 {
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

func (b *Bridge) getAccountNonce(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

	if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) { // increase automatically
		nonce = b.GetSwapNonce(args.From)
		return &nonce, nil
	}

	for i := 0; i < retryRPCCount; i++ {
		nonce, err = b.GetPoolNonce(args.From, "pending")
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err != nil {
		return nil, err
	}
	nonce = b.AdjustNonce(args.From, nonce)
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


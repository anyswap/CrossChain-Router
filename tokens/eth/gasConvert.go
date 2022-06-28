package eth

import (
	"bytes"
	"errors"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

// router contract's log topics
var (
	// LogGasConvertOut(address from, string to, uint256 amount, uint256 fromChainID, uint256 toChainID);
	LogGasConvertOut     = common.FromHex("0x524033a372247e0bfbc5115a280f31c0c70e7494547fb6c915ed992a9c8bbbdc")
	NativeToken          = "0x0000000000000000000000000000000000000000"
	GasConvertInFuncHash = common.FromHex("0x0ac4f61f")
)

func (b *Bridge) registerGasConvertTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	commonInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{GasConvertSwapInfo: &tokens.GasConvertSwapInfo{}}}
	commonInfo.SwapType = tokens.GasConvertSwapType // SwapType
	commonInfo.Hash = strings.ToLower(txHash)       // Hash
	commonInfo.LogIndex = logIndex                  // LogIndex

	receipt, err := b.getSwapTxReceipt(commonInfo, true)
	if err != nil {
		return []*tokens.SwapTxInfo{commonInfo}, []error{err}
	}

	swapInfos := make([]*tokens.SwapTxInfo, 0)
	errs := make([]error, 0)
	startIndex, endIndex := 0, len(receipt.Logs)

	if logIndex != 0 {
		if logIndex >= endIndex || logIndex < 0 {
			return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrLogIndexOutOfRange}
		}
		startIndex = logIndex
		endIndex = logIndex + 1
	}

	for i := startIndex; i < endIndex; i++ {
		swapInfo := &tokens.SwapTxInfo{}
		*swapInfo = *commonInfo
		swapInfo.LogIndex = i // LogIndex
		err := b.verifyGasConvertTxLog(swapInfo, receipt.Logs[i])
		switch {
		case errors.Is(err, tokens.ErrSwapoutLogNotFound),
			errors.Is(err, tokens.ErrTxWithWrongTopics),
			errors.Is(err, tokens.ErrTxWithWrongContract):
			continue
		case err == nil:
			err = b.checkGasConvertInfo(swapInfo)
		default:
			log.Debug(b.ChainConfig.BlockChain+" register gasConvert swap error", "txHash", txHash, "logIndex", swapInfo.LogIndex, "err", err)
		}
		swapInfos = append(swapInfos, swapInfo)
		errs = append(errs, err)
	}

	if len(swapInfos) == 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrSwapoutLogNotFound}
	}

	return swapInfos, errs
}

func (b *Bridge) verifyGasConvertTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	swapInfo.To = rlog.Address.LowerHex() // To
	logTopic := rlog.Topics[0].Bytes()
	switch {
	case bytes.Equal(logTopic, LogGasConvertOut):
		err = b.parseGasConvertTxLog(swapInfo, rlog)
	default:
		return tokens.ErrSwapoutLogNotFound
	}
	if err != nil {
		log.Info(b.ChainConfig.BlockChain+" verifyERC20SwapTxLog fail", "tx", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", err)
		return err
	}

	if rlog.Removed != nil && *rlog.Removed {
		return tokens.ErrTxWithRemovedLog
	}

	routerContract := b.GetRouterContract(swapInfo.GasConvertSwapInfo.Token)
	if routerContract == "" {
		return tokens.ErrMissRouterInfo
	}
	if !common.IsEqualIgnoreCase(rlog.Address.LowerHex(), routerContract) {
		log.Warn("router contract mismatch", "have", rlog.Address.LowerHex(), "want", routerContract)
		return tokens.ErrTxWithWrongContract
	}
	return nil
}

func (b *Bridge) parseGasConvertTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 3 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) < 96 {
		return abicoder.ErrParseDataError
	}

	gasConvertSwapInfo := swapInfo.GasConvertSwapInfo
	gasConvertSwapInfo.Token = NativeToken
	swapInfo.From = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.Bind, err = abicoder.ParseStringInData(logData, 0)
	if err != nil {
		return err
	}
	swapInfo.Value = common.GetBigInt(logData, 32, 32)
	if params.IsUseFromChainIDInReceiptDisabled(b.ChainConfig.ChainID) {
		swapInfo.FromChainID = b.ChainConfig.GetChainID()
	} else {
		swapInfo.FromChainID = common.GetBigInt(logData, 64, 32)
	}
	swapInfo.ToChainID, err = common.GetBigIntFromStr(logTopics[2].String())
	if err != nil {
		return err
	}
	tokenCfg := b.GetTokenConfig(gasConvertSwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	gasConvertSwapInfo.TokenID = tokenCfg.TokenID
	log.Warn("parseGasConvertTxLog", "tokenCfg.TokenID", tokenCfg.TokenID, "swapInfo.Value", swapInfo.Value, "swapInfo.Bind", swapInfo.Bind, "swapInfo.From", swapInfo.From)

	return nil
}

func (b *Bridge) checkGasConvertInfo(swapInfo *tokens.SwapTxInfo) error {
	if swapInfo.FromChainID.String() != b.ChainConfig.ChainID {
		log.Error("router gasConvert tx with mismatched fromChainID in receipt", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID, "chainID", b.ChainConfig.ChainID)
		return tokens.ErrFromChainIDMismatch
	}

	dstBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if dstBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	if !dstBridge.IsValidAddress(swapInfo.Bind) {
		log.Warn("wrong bind address in erc20 swap", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "bind", swapInfo.Bind)
		return tokens.ErrWrongBindAddress
	}
	return nil
}

func (b *Bridge) verifyGasConvertTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{GasConvertSwapInfo: &tokens.GasConvertSwapInfo{}}}
	swapInfo.SwapType = tokens.GasConvertSwapType // SwapType
	swapInfo.Hash = strings.ToLower(txHash)       // Hash
	swapInfo.LogIndex = logIndex                  // LogIndex

	receipt, err := b.getSwapTxReceipt(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	if logIndex >= len(receipt.Logs) {
		return swapInfo, tokens.ErrLogIndexOutOfRange
	}

	err = b.verifyGasConvertTxLog(swapInfo, receipt.Logs[logIndex])
	if err != nil {
		return swapInfo, err
	}

	err = b.checkGasConvertInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		ctx := []interface{}{
			"identifier", params.GetIdentifier(),
			"from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value,
			"txid", txHash, "logIndex", logIndex,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp,
			"fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID,
			"token", swapInfo.GasConvertSwapInfo.Token, "tokenID", swapInfo.GasConvertSwapInfo.TokenID,
		}
		log.Info("verify router swap tx stable pass", ctx...)
	}

	return swapInfo, nil
}

func (b *Bridge) buildGasConvertTxInput(args *tokens.BuildTxArgs) (err error) {
	//todo get currencySymbol by chainID
	currencySymbol := "ethereum"
	price, err := GetNativePrice(currencySymbol)
	if err != nil {
		return err
	}

	destCurrencySymbol := "ethereum"
	destPrice, err := GetNativePrice(destCurrencySymbol)
	if err != nil {
		return err
	}
	log.Warn("buildGasConvertTxInput", "price", price, "destPrice", destPrice)

	priceRate := big.NewFloat(price / destPrice)
	floatAmount := new(big.Float).SetInt(args.OriginValue)
	result, accuracy := priceRate.Mul(priceRate, floatAmount).Int64()
	amount := big.NewInt(result)
	log.Warn("buildGasConvertTxInput", "priceRate", priceRate, "floatAmount", floatAmount, "amount", amount, "accuracy", accuracy, "result", result)

	input := abicoder.PackDataWithFuncHash(GasConvertInFuncHash,
		common.HexToHash(args.SwapID),
		common.HexToAddress(args.Bind),
		amount,
		args.FromChainID,
	)
	args.Input = (*hexutil.Bytes)(&input)                        // input
	args.To = b.GetRouterContract(args.GasConvertSwapInfo.Token) // to
	args.SwapValue = amount                                      // swapValue

	return nil

}

func GetNativePrice(currencySymbol string) (float64, error) {
	type coinsData struct {
		Price float64 `json:"current_price"`
	}
	var result []coinsData
	restApi := "https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=" + currencySymbol
	err := client.RPCGet(&result, restApi)
	if err != nil || len(result) == 0 {
		return 0, err
	}
	if err != nil {
		return 0, err
	}
	return result[0].Price, nil
}

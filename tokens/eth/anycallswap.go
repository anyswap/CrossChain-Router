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
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

// anycall lot topics and func hashes
var (
	// v5 LogAnyCall(address,address,bytes,address,uint256)
	LogAnyCallV5Topic = common.FromHex("0x9ca1de98ebed0a9c38ace93d3ca529edacbbe199cf1b6f0f416ae9b724d4a81c")
	AnyExecV5FuncHash = common.FromHex("0xb4c5dbd0")

	// v6 LogAnyCall(address,address,bytes,address,uint256,uint256,string,uint256)
	LogAnyCallV6Topic = common.FromHex("0xa17aef042e1a5dd2b8e68f0d0d92f9a6a0b35dc25be1d12c0cb3135bfd8951c9")
	AnyExecV6FuncHash = common.FromHex("0x4a578150")
	// anyFallback(address,bytes)
	AnyExecV6FallbackFuncHash = common.FromHex("0xa35fe8bf")

	defMinReserveBudget = big.NewInt(1e16)
)

func getAnyCallLogTopic() ([]byte, error) {
	switch params.GetSwapSubType() {
	case tokens.AnycallSubTypeV5, tokens.CurveAnycallSubType:
		return LogAnyCallV5Topic, nil
	case tokens.AnycallSubTypeV6:
		return LogAnyCallV6Topic, nil
	default:
		return nil, tokens.ErrUnknownSwapSubType
	}
}

func getAnyExecFuncHash() ([]byte, error) {
	switch params.GetSwapSubType() {
	case tokens.AnycallSubTypeV5, tokens.CurveAnycallSubType:
		return AnyExecV5FuncHash, nil
	case tokens.AnycallSubTypeV6:
		return AnyExecV6FuncHash, nil
	default:
		return nil, tokens.ErrUnknownSwapSubType
	}
}

func (b *Bridge) registerAnyCallSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	commonInfo := &tokens.SwapTxInfo{}
	commonInfo.SwapType = tokens.AnyCallSwapType        // SwapType
	commonInfo.Hash = strings.ToLower(txHash)           // Hash
	commonInfo.LogIndex = logIndex                      // LogIndex
	commonInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

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
		err := b.verifyAnyCallSwapTxLog(swapInfo, receipt.Logs[i])
		switch {
		case errors.Is(err, tokens.ErrSwapoutLogNotFound),
			errors.Is(err, tokens.ErrTxWithWrongTopics),
			errors.Is(err, tokens.ErrTxWithWrongContract):
			continue
		case err == nil:
			err = b.checkAnyCallSwapInfo(swapInfo)
		default:
			log.Debug(b.ChainConfig.BlockChain+" register anycall swap error", "txHash", txHash, "logIndex", swapInfo.LogIndex, "err", err)
		}
		swapInfos = append(swapInfos, swapInfo)
		errs = append(errs, err)
	}

	if len(swapInfos) == 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrSwapoutLogNotFound}
	}

	return swapInfos, errs
}

func (b *Bridge) verifyAnyCallSwapTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.AnyCallSwapType        // SwapType
	swapInfo.Hash = strings.ToLower(txHash)           // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	receipt, err := b.getSwapTxReceipt(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	if logIndex >= len(receipt.Logs) {
		return swapInfo, tokens.ErrLogIndexOutOfRange
	}

	err = b.verifyAnyCallSwapTxLog(swapInfo, receipt.Logs[logIndex])
	if err != nil {
		return swapInfo, err
	}

	err = b.checkAnyCallSwapInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify anycall swap tx stable pass", "identifier", params.GetIdentifier(),
			"from", swapInfo.From, "to", swapInfo.To, "txid", txHash, "logIndex", logIndex,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp,
			"fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID,
			"callFrom", getCallFrom(swapInfo))
	}

	return swapInfo, nil
}

func getCallFrom(swapInfo *tokens.SwapTxInfo) string {
	return swapInfo.AnyCallSwapInfo.CallFrom
}

func (b *Bridge) verifyAnyCallSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	swapInfo.To = rlog.Address.LowerHex() // To

	err = b.parseAnyCallSwapTxLog(swapInfo, rlog)
	if err != nil {
		log.Info(b.ChainConfig.BlockChain+" b.verifyAnyCallSwapTxLog fail", "tx", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", err)
		return err
	}

	if rlog.Removed != nil && *rlog.Removed {
		return tokens.ErrTxWithRemovedLog
	}

	routerContract := b.GetRouterContract("")
	if !common.IsEqualIgnoreCase(rlog.Address.LowerHex(), routerContract) {
		log.Warn("tx to address mismatch", "have", rlog.Address.LowerHex(), "want", routerContract, "chainID", b.ChainConfig.ChainID, "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", tokens.ErrTxWithWrongContract)
		return tokens.ErrTxWithWrongContract
	}
	return nil
}

func (b *Bridge) parseAnyCallSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	anycallLogTopic, err := getAnyCallLogTopic()
	if err != nil {
		return err
	}
	logTopic := rlog.Topics[0].Bytes()
	if !bytes.Equal(logTopic, anycallLogTopic) {
		return tokens.ErrSwapoutLogNotFound
	}

	logData := *rlog.Data
	if len(logData) < 96 {
		return abicoder.ErrParseDataError
	}

	swapInfo.SwapInfo = tokens.SwapInfo{AnyCallSwapInfo: &tokens.AnyCallSwapInfo{}}
	anycallSwapInfo := swapInfo.AnyCallSwapInfo

	anycallSwapInfo.CallFrom = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	anycallSwapInfo.CallTo = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.ToChainID = new(big.Int).SetBytes(logTopics[3].Bytes())

	anycallSwapInfo.CallData, err = abicoder.ParseBytesInData(logData, 0)
	if err != nil {
		return err
	}
	anycallSwapInfo.Fallback = common.BytesToAddress(common.GetData(logData, 32, 32)).LowerHex()

	if params.GetSwapSubType() == tokens.AnycallSubTypeV5 || params.GetSwapSubType() == tokens.CurveAnycallSubType {
		return nil
	}

	if len(logData) < 224 {
		return abicoder.ErrParseDataError
	}

	anycallSwapInfo.Flags = common.GetBigInt(logData, 64, 32).String()
	anycallSwapInfo.AppID, err = abicoder.ParseStringInData(logData, 96)
	if err != nil {
		return err
	}
	anycallSwapInfo.Nonce = common.GetBigInt(logData, 128, 32).String()

	// ignore configed anycall v6 apps which do not support fallback
	if params.IsAnycallFallbackIgnored(anycallSwapInfo.AppID) &&
		len(anycallSwapInfo.CallData) >= 100 &&
		bytes.Equal(anycallSwapInfo.CallData[:4], AnyExecV6FallbackFuncHash) &&
		strings.EqualFold(anycallSwapInfo.CallFrom, anycallSwapInfo.CallTo) &&
		common.HexToAddress(anycallSwapInfo.Fallback) == (common.Address{}) &&
		anycallSwapInfo.Flags == "0" {
		return tokens.ErrFallbackNotSupport
	}

	return nil
}

func (b *Bridge) checkAnyCallSwapInfo(swapInfo *tokens.SwapTxInfo) error {
	if swapInfo.FromChainID.String() != b.ChainConfig.ChainID {
		log.Error("anycall swap tx with mismatched fromChainID in receipt", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID, "chainID", b.ChainConfig.ChainID)
		return tokens.ErrFromChainIDMismatch
	}
	if swapInfo.FromChainID.Cmp(swapInfo.ToChainID) == 0 {
		return tokens.ErrSameFromAndToChainID
	}
	dstBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if dstBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	// check budget on dest chain to prvent DOS attack
	if params.HasMinReserveBudgetConfig() {
		minReserveBudget := params.GetMinReserveBudget(dstBridge.GetChainConfig().ChainID)
		if minReserveBudget == nil {
			minReserveBudget = defMinReserveBudget
		}
		callFrom := getCallFrom(swapInfo)
		if err := tokens.CheckNativeBalance(dstBridge, callFrom, minReserveBudget); err != nil {
			return tokens.ErrNoEnoughReserveBudget
		}
	}
	return nil
}

func (b *Bridge) buildAnyCallSwapTxInput(args *tokens.BuildTxArgs) (err error) {
	if b.ChainConfig.ChainID != args.ToChainID.String() {
		return errors.New("anycall to chainId mismatch")
	}

	anycallSwapInfo := args.AnyCallSwapInfo
	if anycallSwapInfo == nil {
		return errors.New("build anycall swaptx without swapinfo")
	}

	funcHash, err := getAnyExecFuncHash()
	if err != nil {
		return err
	}

	var input []byte
	switch params.GetSwapSubType() {
	case tokens.AnycallSubTypeV5, tokens.CurveAnycallSubType:
		input = abicoder.PackDataWithFuncHash(funcHash,
			common.HexToAddress(anycallSwapInfo.CallFrom),
			common.HexToAddress(anycallSwapInfo.CallTo),
			anycallSwapInfo.CallData,
			common.HexToAddress(anycallSwapInfo.Fallback),
			args.FromChainID,
		)
	case tokens.AnycallSubTypeV6:
		nonce, err := common.GetBigIntFromStr(anycallSwapInfo.Nonce)
		if err != nil {
			return err
		}
		flags, err := common.GetBigIntFromStr(anycallSwapInfo.Flags)
		if err != nil {
			return err
		}
		input = abicoder.PackDataWithFuncHash(funcHash,
			common.HexToAddress(anycallSwapInfo.CallTo),
			anycallSwapInfo.CallData,
			common.HexToAddress(anycallSwapInfo.Fallback),
			anycallSwapInfo.AppID,
			common.HexToHash(args.SwapID),
			common.HexToAddress(anycallSwapInfo.CallFrom),
			args.FromChainID,
			nonce,
			flags,
		)
	}

	args.Input = (*hexutil.Bytes)(&input) // input

	routerContract := b.GetRouterContract("")
	args.To = routerContract // to

	return nil
}

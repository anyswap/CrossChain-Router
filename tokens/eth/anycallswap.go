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
	// LogAnyCall(address,address[],bytes[],address[],uint256[],uint256,uint256)
	LogAnyCallTopic = common.FromHex("0x3d1b3d059223895589208a5541dce543eab6d5942b3b1129231a942d1c47bc45")
	AnyExecFuncHash = common.FromHex("0x32f29022")

	// LogAnyCall(address,address,bytes,address,uint256)
	LogCurveAnyCallTopic = common.FromHex("0x9ca1de98ebed0a9c38ace93d3ca529edacbbe199cf1b6f0f416ae9b724d4a81c")
	CurveAnyExecFuncHash = common.FromHex("0xb4c5dbd0")
)

const (
	curveAnycallSubType = "curve"
)

// nolint:dupl // ok
func (b *Bridge) registerAnyCallSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	commonInfo := &tokens.SwapTxInfo{}
	commonInfo.SwapType = tokens.AnyCallSwapType // SwapType
	commonInfo.Hash = strings.ToLower(txHash)    // Hash
	commonInfo.LogIndex = logIndex               // LogIndex

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
	swapInfo.SwapType = tokens.AnyCallSwapType // SwapType
	swapInfo.Hash = strings.ToLower(txHash)    // Hash
	swapInfo.LogIndex = logIndex               // LogIndex

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
	switch params.GetSwapSubType() {
	case curveAnycallSubType:
		return swapInfo.CurveAnyCallSwapInfo.CallFrom
	default:
		return swapInfo.AnyCallSwapInfo.CallFrom
	}
}

func (b *Bridge) verifyAnyCallSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	swapInfo.To = rlog.Address.LowerHex() // To

	switch params.GetSwapSubType() {
	case curveAnycallSubType:
		err = b.parseCurveAnyCallSwapTxLog(swapInfo, rlog)
	default:
		err = b.parseAnyCallSwapTxLog(swapInfo, rlog)
	}
	if err != nil {
		log.Info(b.ChainConfig.BlockChain+" b.verifyAnyCallSwapTxLog fail", "tx", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", err)
		return err
	}

	if rlog.Removed != nil && *rlog.Removed {
		return tokens.ErrTxWithRemovedLog
	}

	routerContract := b.GetRouterContract("")
	if !common.IsEqualIgnoreCase(rlog.Address.LowerHex(), routerContract) {
		log.Warn("swap tx with wrong contract", "log.Address", rlog.Address.LowerHex(), "routerContract", routerContract)
		return tokens.ErrTxWithWrongContract
	}
	return nil
}

func (b *Bridge) parseCurveAnyCallSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	logTopic := rlog.Topics[0].Bytes()
	if !bytes.Equal(logTopic, LogCurveAnyCallTopic) {
		return tokens.ErrSwapoutLogNotFound
	}

	logData := *rlog.Data
	if len(logData) < 96 {
		return abicoder.ErrParseDataError
	}

	swapInfo.SwapInfo = tokens.SwapInfo{CurveAnyCallSwapInfo: &tokens.CurveAnyCallSwapInfo{}}
	anycallSwapInfo := swapInfo.CurveAnyCallSwapInfo

	anycallSwapInfo.CallFrom = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	anycallSwapInfo.CallTo = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.ToChainID = new(big.Int).SetBytes(logTopics[3].Bytes())
	swapInfo.FromChainID = b.ChainConfig.GetChainID()

	anycallSwapInfo.CallData, err = abicoder.ParseBytesInData(logData, 0)
	if err != nil {
		return err
	}
	anycallSwapInfo.Fallback = common.BytesToAddress(common.GetData(logData, 32, 32)).LowerHex()
	if err != nil {
		return err
	}
	return nil
}

func (b *Bridge) parseAnyCallSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 2 {
		return tokens.ErrTxWithWrongTopics
	}
	logTopic := rlog.Topics[0].Bytes()
	if !bytes.Equal(logTopic, LogAnyCallTopic) {
		return tokens.ErrSwapoutLogNotFound
	}

	logData := *rlog.Data
	if len(logData) < 320 {
		return abicoder.ErrParseDataError
	}

	swapInfo.SwapInfo = tokens.SwapInfo{AnyCallSwapInfo: &tokens.AnyCallSwapInfo{}}
	anycallSwapInfo := swapInfo.AnyCallSwapInfo

	anycallSwapInfo.CallFrom = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	anycallSwapInfo.CallTo, err = abicoder.ParseAddressSliceInData(logData, 0)
	if err != nil {
		return err
	}
	anycallSwapInfo.CallData, err = abicoder.ParseBytesSliceInData(logData, 32)
	if err != nil {
		return err
	}
	anycallSwapInfo.Callbacks, err = abicoder.ParseAddressSliceInData(logData, 64)
	if err != nil {
		return err
	}
	anycallSwapInfo.CallNonces, err = abicoder.ParseNumberSliceAsBigIntsInData(logData, 96)
	if err != nil {
		return err
	}
	if params.IsUseFromChainIDInReceiptDisabled(b.ChainConfig.ChainID) {
		swapInfo.FromChainID = b.ChainConfig.GetChainID()
	} else {
		swapInfo.FromChainID = common.GetBigInt(logData, 128, 32)
	}
	swapInfo.ToChainID = common.GetBigInt(logData, 160, 32)
	return nil
}

func (b *Bridge) checkAnyCallSwapInfo(swapInfo *tokens.SwapTxInfo) error {
	if swapInfo.FromChainID.String() != b.ChainConfig.ChainID {
		log.Error("anycall swap tx with mismatched fromChainID in receipt", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID, "chainID", b.ChainConfig.ChainID)
		return tokens.ErrFromChainIDMismatch
	}
	dstBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if dstBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	return nil
}

func (b *Bridge) buildAnyCallSwapTxInput(args *tokens.BuildTxArgs) (err error) {
	if b.ChainConfig.ChainID != args.ToChainID.String() {
		return errors.New("anycall to chainId mismatch")
	}

	var input []byte
	switch params.GetSwapSubType() {
	case curveAnycallSubType:
		funcHash := CurveAnyExecFuncHash
		anycallSwapInfo := args.CurveAnyCallSwapInfo
		if anycallSwapInfo == nil {
			return errors.New("build anycall swaptx without swapinfo")
		}
		input = abicoder.PackDataWithFuncHash(funcHash,
			common.HexToAddress(anycallSwapInfo.CallFrom),
			common.HexToAddress(anycallSwapInfo.CallTo),
			anycallSwapInfo.CallData,
			common.HexToAddress(anycallSwapInfo.Fallback),
			args.FromChainID,
		)
	default:
		funcHash := AnyExecFuncHash
		anycallSwapInfo := args.AnyCallSwapInfo
		if anycallSwapInfo == nil {
			return errors.New("build anycall swaptx without swapinfo")
		}
		input = abicoder.PackDataWithFuncHash(funcHash,
			common.HexToAddress(anycallSwapInfo.CallFrom),
			toAddresses(anycallSwapInfo.CallTo),
			anycallSwapInfo.CallData,
			toAddresses(anycallSwapInfo.Callbacks),
			anycallSwapInfo.CallNonces,
			args.FromChainID,
		)
	}

	args.Input = (*hexutil.Bytes)(&input) // input

	routerContract := b.GetRouterContract("")
	args.To = routerContract // to

	return nil
}

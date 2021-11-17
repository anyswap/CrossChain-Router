package eth

import (
	"bytes"
	"errors"
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
	LogAnyCallTopic = common.FromHex("0x3d1b3d059223895589208a5541dce543eab6d5942b3b1129231a942d1c47bc45")

	AnyCallFuncHash = common.FromHex("0x32f29022")
)

// nolint:dupl // ok
func (b *Bridge) registerAnyCallSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	commonInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{AnyCallSwapInfo: &tokens.AnyCallSwapInfo{}}}
	commonInfo.SwapType = tokens.AnyCallSwapType // SwapType
	commonInfo.Hash = strings.ToLower(txHash)    // Hash
	commonInfo.LogIndex = logIndex               // LogIndex

	receipt, err := b.getAndVerifySwapTxReceipt(commonInfo, true)
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
		swapInfo.AnyCallSwapInfo = &tokens.AnyCallSwapInfo{}
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
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{AnyCallSwapInfo: &tokens.AnyCallSwapInfo{}}}
	swapInfo.SwapType = tokens.AnyCallSwapType // SwapType
	swapInfo.Hash = strings.ToLower(txHash)    // Hash
	swapInfo.LogIndex = logIndex               // LogIndex

	receipt, err := b.getAndVerifySwapTxReceipt(swapInfo, allowUnstable)
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
			"callFrom", swapInfo.AnyCallSwapInfo.CallFrom)
	}

	return swapInfo, nil
}

func (b *Bridge) verifyAnyCallSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	swapInfo.To = rlog.Address.LowerHex() // To
	if !common.IsEqualIgnoreCase(rlog.Address.LowerHex(), b.ChainConfig.RouterContract) {
		return tokens.ErrTxWithWrongContract
	}

	logTopic := rlog.Topics[0].Bytes()
	if !bytes.Equal(logTopic, LogAnyCallTopic) {
		return tokens.ErrSwapoutLogNotFound
	}

	err = b.parseAnyCallSwapTxLog(swapInfo, rlog)
	if err != nil {
		log.Info(b.ChainConfig.BlockChain+" b.verifyAnyCallSwapTxLog fail", "tx", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", err)
		return err
	}

	if rlog.Removed != nil && *rlog.Removed {
		return tokens.ErrTxWithRemovedLog
	}
	return nil
}

func (b *Bridge) parseAnyCallSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 2 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) < 320 {
		return abicoder.ErrParseDataError
	}

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
	if !dstBridge.IsValidAddress(swapInfo.Bind) {
		log.Warn("wrong bind address in anycall swap", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "bind", swapInfo.Bind)
		return tokens.ErrWrongBindAddress
	}
	return nil
}

func (b *Bridge) buildAnyCallSwapTxInput(args *tokens.BuildTxArgs) (err error) {
	if args.AnyCallSwapInfo == nil {
		return errors.New("build anycall swaptx without swapinfo")
	}
	anycallSwapInfo := args.AnyCallSwapInfo
	funcHash := AnyCallFuncHash

	if b.ChainConfig.ChainID != args.ToChainID.String() {
		return errors.New("anycall to chainId mismatch")
	}

	input := abicoder.PackDataWithFuncHash(funcHash,
		common.HexToAddress(anycallSwapInfo.CallFrom),
		toAddresses(anycallSwapInfo.CallTo),
		anycallSwapInfo.CallData,
		toAddresses(anycallSwapInfo.Callbacks),
		anycallSwapInfo.CallNonces,
		args.FromChainID,
	)
	args.Input = (*hexutil.Bytes)(&input)  // input
	args.To = b.ChainConfig.RouterContract // to

	return nil
}

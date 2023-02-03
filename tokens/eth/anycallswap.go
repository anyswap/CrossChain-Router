package eth

import (
	"bytes"
	"errors"
	"fmt"
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

	// v7 LogAnyCall(address,address,bytes,uint256,uint256,string,uint256,bytes)
	LogAnyCallV7Topic = common.FromHex("0x17dac14bf31c4070ebb2dc182fc25ae5df58f14162a7f24a65b103e22385af0d")
	// v7 LogAnyCall(address,string,bytes,uint256,uint256,string,uint256,bytes)
	LogAnyCallV7Topic2 = common.FromHex("0x36850177870d3e3dca07a29dcdc3994356392b81c60f537c1696468b1a01e61d")
	AnyExecV7FuncHash  = common.FromHex("0xd7328bad")

	// anycall usdc
	LogMessageSentTopic = common.FromHex("0x8c5261668696ce22758910d05bab8f186d6eb247ceac2af2e82c7dc17669b036")

	defMinReserveBudget = big.NewInt(1e16)
)

func checkAnyCallLogTopic(logTopic []byte) error {
	var filterTopics [][]byte
	switch params.GetSwapSubType() {
	case tokens.AnycallSubTypeV7:
		filterTopics = [][]byte{LogAnyCallV7Topic, LogAnyCallV7Topic2}
	case tokens.AnycallSubTypeV6:
		filterTopics = [][]byte{LogAnyCallV6Topic}
	case tokens.AnycallSubTypeV5, tokens.CurveAnycallSubType:
		filterTopics = [][]byte{LogAnyCallV5Topic}
	default:
		return tokens.ErrUnknownSwapSubType
	}

	for _, topic := range filterTopics {
		if bytes.Equal(logTopic, topic) {
			return nil
		}
	}

	return tokens.ErrSwapoutLogNotFound
}

func getAnyExecFuncHash() ([]byte, error) {
	switch params.GetSwapSubType() {
	case tokens.AnycallSubTypeV7:
		return AnyExecV7FuncHash, nil
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

	anycallSwapInfo := swapInfo.AnyCallSwapInfo
	isUsdcAnycall := len(anycallSwapInfo.ExtData) == 4 && bytes.Equal(anycallSwapInfo.ExtData, []byte("usdc"))
	if isUsdcAnycall {
		err = b.findMessageSentInfo(swapInfo, receipt.Logs, logIndex, allowUnstable)
		if err != nil {
			return swapInfo, err
		}
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
	if rlog == nil || len(rlog.Topics) == 0 {
		return tokens.ErrSwapoutLogNotFound
	}

	logTopic := rlog.Topics[0].Bytes()
	err = checkAnyCallLogTopic(logTopic)
	if err != nil {
		return err
	}

	switch {
	case bytes.Equal(logTopic, LogAnyCallV7Topic):
		return b.parseAnyCallV7Log(swapInfo, rlog, false)
	case bytes.Equal(logTopic, LogAnyCallV7Topic2):
		return b.parseAnyCallV7Log(swapInfo, rlog, true)
	case bytes.Equal(logTopic, LogAnyCallV6Topic):
		return b.parseAnyCallV6Log(swapInfo, rlog)
	case bytes.Equal(logTopic, LogAnyCallV5Topic):
		return b.parseAnyCallV5Log(swapInfo, rlog)
	default:
		return tokens.ErrSwapoutLogNotFound
	}
}

func (b *Bridge) parseAnyCallV7Log(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog, isStringToAddr bool) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 2 {
		return tokens.ErrTxWithWrongTopics
	}

	var minDataLen int
	if isStringToAddr {
		minDataLen = 352
	} else {
		minDataLen = 320
	}

	logData := *rlog.Data
	if len(logData) < minDataLen {
		return abicoder.ErrParseDataError
	}

	swapInfo.SwapInfo = tokens.SwapInfo{AnyCallSwapInfo: &tokens.AnyCallSwapInfo{}}
	anycallSwapInfo := swapInfo.AnyCallSwapInfo

	anycallSwapInfo.CallFrom = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()

	if isStringToAddr {
		anycallSwapInfo.CallTo, err = abicoder.ParseStringInData(logData, 0)
		if err != nil {
			return err
		}
	} else {
		anycallSwapInfo.CallTo = common.BytesToAddress(common.GetData(logData, 0, 32)).LowerHex()
	}

	anycallSwapInfo.CallData, err = abicoder.ParseBytesInData(logData, 32)
	if err != nil {
		return err
	}

	swapInfo.ToChainID = common.GetBigInt(logData, 64, 32)

	anycallSwapInfo.Flags = common.GetBigInt(logData, 96, 32).String()

	anycallSwapInfo.AppID, err = abicoder.ParseStringInData(logData, 128)
	if err != nil {
		return err
	}

	anycallSwapInfo.Nonce = common.GetBigInt(logData, 160, 32).String()

	anycallSwapInfo.ExtData, err = abicoder.ParseBytesInData(logData, 192)
	if err != nil {
		return err
	}

	return nil
}

func (b *Bridge) parseAnyCallV6Log(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}

	logData := *rlog.Data
	if len(logData) < 224 {
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

func (b *Bridge) parseAnyCallV5Log(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
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

func (b *Bridge) findMessageSentInfo(swapInfo *tokens.SwapTxInfo, logs []*types.RPCLog, logIndex int, allowUnstable bool) error {
	for i := logIndex - 1; i >= 0; i-- {
		rlog := logs[i]
		logTopics := rlog.Topics
		if len(logTopics) == 1 && bytes.Equal(logTopics[0].Bytes(), LogMessageSentTopic) {
			anycallSwapInfo := swapInfo.SwapInfo.AnyCallSwapInfo
			if anycallSwapInfo == nil {
				return errors.New("anycall without swapinfo")
			}
			messageBytes, err := abicoder.ParseBytesInData(*rlog.Data, 0)
			if err != nil {
				return err
			}
			anycallSwapInfo.Message = messageBytes
			messageHash := common.Keccak256Hash(messageBytes)
			log.Info("find message sent info success", "txHash", swapInfo.Hash, "logIndex", logIndex, "msgIndex", i, "message", common.ToHex(messageBytes), "messageHash", messageHash.String())

			if !allowUnstable {
				var attestation *USDCAttestation
				attestation, err = GetUSDCAttestation(messageHash)
				if err != nil {
					return fmt.Errorf("%w. %v %v", tokens.ErrGetAttestationFailed, messageHash.String(), err)
				}
				anycallSwapInfo.Attestation = attestation.Attestation
				log.Info("get attestation success", "txHash", swapInfo.Hash, "logIndex", logIndex, "msgHash", messageHash.String(), "attestation", attestation.Attestation, "status", attestation.Status)
			}
			return nil
		}
	}
	return tokens.ErrMessageSentNotFound
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

	callData := anycallSwapInfo.CallData
	if len(anycallSwapInfo.Attestation) > 0 {
		// format is (bytes _calldata, string _swapid, bytes _message, bytes _attestation)
		callData = abicoder.PackData(
			callData,
			args.GetUniqueSwapIdentifier(),
			anycallSwapInfo.Message,
			anycallSwapInfo.Attestation,
		)
	}

	var input []byte
	switch params.GetSwapSubType() {
	case tokens.AnycallSubTypeV7:
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
			callData,
			anycallSwapInfo.AppID,
			common.HexToHash(args.SwapID),
			common.HexToAddress(anycallSwapInfo.CallFrom),
			args.FromChainID,
			nonce,
			flags,
			anycallSwapInfo.ExtData,
		)
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
	args.SwapValue = big.NewInt(0)

	return nil
}

// USDCAttestation usdc attestation
type USDCAttestation struct {
	Attestation hexutil.Bytes `json:"attestation"`
	Status      string        `json:"status"`
}

// GetUSDCAttestation get usdc attestation
func GetUSDCAttestation(messageHash common.Hash) (*USDCAttestation, error) {
	attestationURL := strings.TrimSuffix(params.GetAttestationServer(), "/")
	if attestationURL == "" {
		return nil, tokens.ErrNoAttestationServer
	}

	url := fmt.Sprintf("%v/attestations/%v", attestationURL, messageHash.String())

	var res *USDCAttestation
	err := client.RPCGetWithTimeout(&res, url, 60)
	return res, err
}

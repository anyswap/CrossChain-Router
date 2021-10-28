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

// nft swap log topics and func hashes
var (
	// LogNFT721SwapOut(address,address,address,uint256,uint256,uint256);
	LogNFT721SwapOutTopic = common.FromHex("0x0d45b0b9f5add3e1bb841982f1fa9303628b0b619b000cb1f9f1c3903329a4c7")
	// LogNFT1155SwapOut(addressindexedtoken,address,address,uint256,uint256,uint256,uint256)
	LogNFT1155SwapOutTopic = common.FromHex("0x5058b8684cf36ffd9f66bc623fbc617a44dd65cf2273306d03d3104af0995cb0")
	//LogNFT1155SwapOutBatch(address,address,address,uint256[],uint256[],uint256,uint256)
	LogNFT1155SwapOutBatchTopic = common.FromHex("0xaa428a5ab688b49b415401782c170d216b33b15711d30cf69482f570eca8db38")

	// nft721SwapIn(bytes32,address,address,uint256,uint256)
	nft721SwapInFuncHash = common.FromHex("09493b23")
	// nft1155SwapIn(bytes32,address,address,uint256,uint256,uint256)
	nft1155SwapInFuncHash = common.FromHex("1b5b36c0")
	// nft1155BatchSwapIn(bytes32,address,address,uint256[],uint256[],uint256)
	nft1155BatchSwapInFuncHash = common.FromHex("88b150f7")

	errWrongIDsOrAmounts = errors.New("wrong ids or amounts")
)

// nft swap with data log topics and func hashes
var (
	// LogNFT721SwapOut(address,address,address,uint256,uint256,uint256,bytes);
	LogNFT721SwapOutWithDataTopic = common.FromHex("0x8ef0d7d8b96825500b3d692d995a543110f8a93f16b7be5d23b5960fd4363bdc")

	// nft721SwapIn(bytes32,address,address,uint256,uint256,bytes)
	nft721SwapInWithDataFuncHash = common.FromHex("2cbba1a4")
)

// nolint:dupl // ok
func (b *Bridge) registerNFTSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	commonInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{NFTSwapInfo: &tokens.NFTSwapInfo{}}}
	commonInfo.SwapType = tokens.NFTSwapType  // SwapType
	commonInfo.Hash = strings.ToLower(txHash) // Hash
	commonInfo.LogIndex = logIndex            // LogIndex

	receipt, err := b.getAndVerifySwapTxReceipt(commonInfo, true)
	if err != nil {
		return []*tokens.SwapTxInfo{commonInfo}, []error{err}
	}

	swapInfos := make([]*tokens.SwapTxInfo, 0)
	errs := make([]error, 0)
	startIndex, endIndex := 1, len(receipt.Logs)

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
		swapInfo.NFTSwapInfo = &tokens.NFTSwapInfo{}
		swapInfo.LogIndex = i // LogIndex
		err := b.verifyNFTSwapTxLog(swapInfo, receipt.Logs[i])
		switch {
		case errors.Is(err, tokens.ErrSwapoutLogNotFound),
			errors.Is(err, tokens.ErrTxWithWrongContract):
			continue
		case err == nil:
			err = b.checkNFTSwapInfo(swapInfo)
		default:
			log.Debug(b.ChainConfig.BlockChain+" register nft swap error", "txHash", txHash, "logIndex", swapInfo.LogIndex, "err", err)
		}
		swapInfos = append(swapInfos, swapInfo)
		errs = append(errs, err)
	}

	if len(swapInfos) == 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrSwapoutLogNotFound}
	}

	return swapInfos, errs
}

func (b *Bridge) verifyNFTSwapTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{NFTSwapInfo: &tokens.NFTSwapInfo{}}}
	swapInfo.SwapType = tokens.NFTSwapType  // SwapType
	swapInfo.Hash = strings.ToLower(txHash) // Hash
	swapInfo.LogIndex = logIndex            // LogIndex

	receipt, err := b.getAndVerifySwapTxReceipt(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	if logIndex >= len(receipt.Logs) {
		return swapInfo, tokens.ErrLogIndexOutOfRange
	}

	err = b.verifyNFTSwapTxLog(swapInfo, receipt.Logs[logIndex])
	if err != nil {
		return swapInfo, err
	}

	err = b.checkNFTSwapInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify nft swap tx stable pass", "identifier", params.GetIdentifier(),
			"from", swapInfo.From, "to", swapInfo.To, "txid", txHash, "logIndex", logIndex,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp,
			"fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID,
			"ids", swapInfo.NFTSwapInfo.IDs,
			"amounts", swapInfo.NFTSwapInfo.Amounts,
			"batch", swapInfo.NFTSwapInfo.Batch)
	}

	return swapInfo, nil
}

func (b *Bridge) verifyNFTSwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	swapInfo.To = rlog.Address.LowerHex() // To
	if !common.IsEqualIgnoreCase(rlog.Address.LowerHex(), b.ChainConfig.RouterContract) {
		return tokens.ErrTxWithWrongContract
	}

	logTopic := rlog.Topics[0].Bytes()
	if params.IsNFTSwapWithData() {
		switch {
		case bytes.Equal(logTopic, LogNFT721SwapOutWithDataTopic):
			err = b.parseNFT721SwapoutWithDataTxLog(swapInfo, rlog)
		default:
			return tokens.ErrSwapoutLogNotFound
		}
	} else {
		switch {
		case bytes.Equal(logTopic, LogNFT721SwapOutTopic):
			err = b.parseNFT721SwapoutTxLog(swapInfo, rlog)
		case bytes.Equal(logTopic, LogNFT1155SwapOutTopic):
			err = b.parseNFT1155SwapOutTxLog(swapInfo, rlog)
		case bytes.Equal(logTopic, LogNFT1155SwapOutBatchTopic):
			err = b.parseNFT1155SwapOutBatchTxLog(swapInfo, rlog)
		default:
			return tokens.ErrSwapoutLogNotFound
		}
	}

	if err != nil {
		log.Info(b.ChainConfig.BlockChain+" b.verifyNFTSwapTxLog fail", "tx", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", err)
		return err
	}

	if rlog.Removed != nil && *rlog.Removed {
		return tokens.ErrTxWithRemovedLog
	}
	return nil
}

func (b *Bridge) parseNFT721SwapoutTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) error {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) != 96 {
		return abicoder.ErrParseDataError
	}
	nftSwapInfo := swapInfo.NFTSwapInfo
	nftSwapInfo.Token = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.Bind = common.BytesToAddress(logTopics[3].Bytes()).LowerHex()
	swapInfo.Value = big.NewInt(0)
	nftSwapInfo.IDs = []*big.Int{common.GetBigInt(logData, 0, 32)}
	if params.IsUseFromChainIDInReceiptDisabled(b.ChainConfig.ChainID) {
		swapInfo.FromChainID = b.ChainConfig.GetChainID()
	} else {
		swapInfo.FromChainID = common.GetBigInt(logData, 32, 32)
	}
	swapInfo.ToChainID = common.GetBigInt(logData, 64, 32)

	tokenCfg := b.GetTokenConfig(nftSwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	nftSwapInfo.TokenID = tokenCfg.TokenID

	return nil
}

func (b *Bridge) parseNFT721SwapoutWithDataTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) < 160 {
		return abicoder.ErrParseDataError
	}
	nftSwapInfo := swapInfo.NFTSwapInfo
	nftSwapInfo.Token = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.Bind = common.BytesToAddress(logTopics[3].Bytes()).LowerHex()
	swapInfo.Value = big.NewInt(0)
	nftSwapInfo.IDs = []*big.Int{common.GetBigInt(logData, 0, 32)}
	swapInfo.FromChainID = common.GetBigInt(logData, 32, 32)
	swapInfo.ToChainID = common.GetBigInt(logData, 64, 32)
	nftSwapInfo.Data, err = abicoder.ParseBytesInData(logData, 96)
	if err != nil {
		return err
	}

	tokenCfg := b.GetTokenConfig(nftSwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	nftSwapInfo.TokenID = tokenCfg.TokenID

	return nil
}

func (b *Bridge) parseNFT1155SwapOutTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) error {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) != 128 {
		return abicoder.ErrParseDataError
	}
	nftSwapInfo := swapInfo.NFTSwapInfo
	nftSwapInfo.Token = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.Bind = common.BytesToAddress(logTopics[3].Bytes()).LowerHex()
	swapInfo.Value = big.NewInt(0)
	nftSwapInfo.IDs = []*big.Int{common.GetBigInt(logData, 0, 32)}
	nftSwapInfo.Amounts = []*big.Int{common.GetBigInt(logData, 32, 32)}
	if params.IsUseFromChainIDInReceiptDisabled(b.ChainConfig.ChainID) {
		swapInfo.FromChainID = b.ChainConfig.GetChainID()
	} else {
		swapInfo.FromChainID = common.GetBigInt(logData, 64, 32)
	}
	swapInfo.ToChainID = common.GetBigInt(logData, 96, 32)

	tokenCfg := b.GetTokenConfig(nftSwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	nftSwapInfo.TokenID = tokenCfg.TokenID

	return nil
}

func (b *Bridge) parseNFT1155SwapOutBatchTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) < 192 {
		return abicoder.ErrParseDataError
	}
	nftSwapInfo := swapInfo.NFTSwapInfo
	nftSwapInfo.Batch = true
	nftSwapInfo.Token = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.Bind = common.BytesToAddress(logTopics[3].Bytes()).LowerHex()
	swapInfo.Value = big.NewInt(0)
	nftSwapInfo.IDs, err = abicoder.ParseNumberSliceAsBigIntsInData(logData, 0)
	if err != nil {
		return err
	}
	nftSwapInfo.Amounts, err = abicoder.ParseNumberSliceAsBigIntsInData(logData, 32)
	if err != nil {
		return err
	}
	if params.IsUseFromChainIDInReceiptDisabled(b.ChainConfig.ChainID) {
		swapInfo.FromChainID = b.ChainConfig.GetChainID()
	} else {
		swapInfo.FromChainID = common.GetBigInt(logData, 64, 32)
	}
	swapInfo.ToChainID = common.GetBigInt(logData, 96, 32)

	tokenCfg := b.GetTokenConfig(nftSwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	nftSwapInfo.TokenID = tokenCfg.TokenID

	return nil
}

func (b *Bridge) checkNFTSwapInfo(swapInfo *tokens.SwapTxInfo) error {
	if swapInfo.FromChainID.String() != b.ChainConfig.ChainID {
		log.Error("nft swap tx with mismatched fromChainID in receipt", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID, "chainID", b.ChainConfig.ChainID)
		return tokens.ErrFromChainIDMismatch
	}
	nftSwapInfo := swapInfo.NFTSwapInfo
	dstBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if dstBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	if !dstBridge.IsValidAddress(swapInfo.Bind) {
		log.Warn("wrong bind address in nft swap", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "bind", swapInfo.Bind)
		return tokens.ErrWrongBindAddress
	}

	fromTokenCfg := b.GetTokenConfig(nftSwapInfo.Token)
	if fromTokenCfg == nil || nftSwapInfo.TokenID == "" {
		return tokens.ErrMissTokenConfig
	}
	multichainToken := router.GetCachedMultichainToken(nftSwapInfo.TokenID, swapInfo.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", nftSwapInfo.TokenID, "chainID", swapInfo.ToChainID)
		return tokens.ErrMissTokenConfig
	}

	switch {
	case nftSwapInfo.Batch:
		if len(nftSwapInfo.IDs) != len(nftSwapInfo.Amounts) || len(nftSwapInfo.IDs) == 0 {
			return errWrongIDsOrAmounts
		}
	case len(nftSwapInfo.Amounts) > 0:
		if len(nftSwapInfo.IDs) != 1 || len(nftSwapInfo.Amounts) != 1 {
			return errWrongIDsOrAmounts
		}
	default:
		if len(nftSwapInfo.IDs) != 1 || len(nftSwapInfo.Amounts) != 0 {
			return errWrongIDsOrAmounts
		}
	}
	return nil
}

func (b *Bridge) buildNFTSwapTxInput(args *tokens.BuildTxArgs) (err error) {
	nftSwapInfo := args.NFTSwapInfo
	if nftSwapInfo == nil {
		return errors.New("build nft swaptx without swapinfo")
	}

	if b.ChainConfig.ChainID != args.ToChainID.String() {
		return errors.New("nftswap to chainId mismatch")
	}

	receiver := common.HexToAddress(args.Bind)
	if receiver == (common.Address{}) || !common.IsHexAddress(args.Bind) {
		log.Warn("nft swapout to wrong receiver", "receiver", args.Bind)
		return tokens.ErrWrongBindAddress
	}

	multichainToken := router.GetCachedMultichainToken(nftSwapInfo.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", nftSwapInfo.TokenID, "chainID", args.ToChainID)
		return tokens.ErrMissTokenConfig
	}

	var input []byte

	switch {
	case nftSwapInfo.Batch:
		if len(nftSwapInfo.IDs) != len(nftSwapInfo.Amounts) || len(nftSwapInfo.IDs) == 0 {
			return errWrongIDsOrAmounts
		}
		input = abicoder.PackDataWithFuncHash(nft1155BatchSwapInFuncHash,
			common.HexToHash(args.SwapID),
			common.HexToAddress(multichainToken),
			receiver,
			nftSwapInfo.IDs,
			nftSwapInfo.Amounts,
			args.FromChainID,
		)
	case len(nftSwapInfo.Amounts) > 0:
		if len(nftSwapInfo.IDs) != 1 || len(nftSwapInfo.Amounts) != 1 {
			return errWrongIDsOrAmounts
		}
		input = abicoder.PackDataWithFuncHash(nft1155SwapInFuncHash,
			common.HexToHash(args.SwapID),
			common.HexToAddress(multichainToken),
			receiver,
			nftSwapInfo.IDs[0],
			nftSwapInfo.Amounts[0],
			args.FromChainID,
		)
	default:
		if len(nftSwapInfo.IDs) != 1 || len(nftSwapInfo.Amounts) != 0 {
			return errWrongIDsOrAmounts
		}
		if params.IsNFTSwapWithData() {
			input = abicoder.PackDataWithFuncHash(nft721SwapInWithDataFuncHash,
				common.HexToHash(args.SwapID),
				common.HexToAddress(multichainToken),
				receiver,
				nftSwapInfo.IDs[0],
				args.FromChainID,
				nftSwapInfo.Data,
			)
		} else {
			input = abicoder.PackDataWithFuncHash(nft721SwapInFuncHash,
				common.HexToHash(args.SwapID),
				common.HexToAddress(multichainToken),
				receiver,
				nftSwapInfo.IDs[0],
				args.FromChainID,
			)
		}
	}

	args.Input = (*hexutil.Bytes)(&input)  // input
	args.To = b.ChainConfig.RouterContract // to

	return nil
}

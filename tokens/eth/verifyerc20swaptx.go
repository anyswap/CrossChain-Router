package eth

import (
	"bytes"
	"errors"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

// router contract's log topics
var (
	// LogAnySwapOut(address token, address from, address to, uint amount, uint fromChainID, uint toChainID);
	LogAnySwapOutTopic = common.FromHex("0x97116cf6cd4f6412bb47914d6db18da9e16ab2142f543b86e207c24fbd16b23a")
	// LogAnySwapTradeTokensForTokens(address[] path, address from, address to, uint amountIn, uint amountOutMin, uint fromChainID, uint toChainID);
	LogAnySwapTradeTokensForTokensTopic = common.FromHex("0xfea6abdf4fd32f20966dff7619354cd82cd43dc78a3bee479f04c74dbfc585b3")
	// LogAnySwapTradeTokensForNative(address[] path, address from, address to, uint amountIn, uint amountOutMin, uint fromChainID, uint toChainID);
	LogAnySwapTradeTokensForNativeTopic = common.FromHex("0x278277e0209c347189add7bd92411973b5f6b8644f7ac62ea1be984ce993f8f4")
)

func (b *Bridge) verifyERC20SwapTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType // SwapType
	swapInfo.Hash = strings.ToLower(txHash)  // Hash
	swapInfo.LogIndex = logIndex             // LogIndex

	receipt, err := b.getAndVerifySwapTxReceipt(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	if logIndex >= len(receipt.Logs) {
		return swapInfo, tokens.ErrLogIndexOutOfRange
	}

	err = b.verifyERC20SwapTxLog(swapInfo, receipt.Logs[logIndex])
	if err != nil {
		return swapInfo, err
	}

	err = b.checkERC20SwapInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify router swap tx stable pass", "identifier", params.GetIdentifier(),
			"from", swapInfo.From, "to", swapInfo.To, "bind", swapInfo.Bind, "value", swapInfo.Value,
			"txid", txHash, "logIndex", logIndex, "height", swapInfo.Height, "timestamp", swapInfo.Timestamp,
			"fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID,
			"token", swapInfo.ERC20SwapInfo.Token,
			"tokenID", swapInfo.ERC20SwapInfo.TokenID,
			"forNative", swapInfo.ERC20SwapInfo.ForNative,
			"forUnderlying", swapInfo.ERC20SwapInfo.ForUnderlying)
	}

	return swapInfo, nil
}

func (b *Bridge) checkERC20SwapInfo(swapInfo *tokens.SwapTxInfo) error {
	if swapInfo.FromChainID.String() != b.ChainConfig.ChainID {
		log.Error("router swap tx with mismatched fromChainID in receipt", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID, "chainID", b.ChainConfig.ChainID)
		return tokens.ErrFromChainIDMismatch
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	fromTokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil || erc20SwapInfo.TokenID == "" {
		return tokens.ErrMissTokenConfig
	}
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, swapInfo.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", swapInfo.ToChainID)
		return tokens.ErrMissTokenConfig
	}
	toBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if toBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	toTokenCfg := toBridge.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		log.Warn("get token config failed", "chainID", swapInfo.ToChainID, "token", multichainToken)
		return tokens.ErrMissTokenConfig
	}
	if erc20SwapInfo.ForUnderlying && toTokenCfg.GetUnderlying() == (common.Address{}) {
		return tokens.ErrNoUnderlyingToken
	}
	if !tokens.CheckTokenSwapValue(swapInfo, fromTokenCfg.Decimals, toTokenCfg.Decimals) {
		return tokens.ErrTxWithWrongValue
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

func (b *Bridge) getAndVerifySwapTxReceipt(swapInfo *tokens.SwapTxInfo, allowUnstable bool) (receipt *types.RPCTxReceipt, err error) {
	receipt, err = b.getSwapTxReceipt(swapInfo, allowUnstable)
	if err != nil {
		return receipt, err
	}
	err = b.verifySwapTxReceipt(swapInfo, receipt)
	return receipt, err
}

func (b *Bridge) getSwapTxReceipt(swapInfo *tokens.SwapTxInfo, allowUnstable bool) (receipt *types.RPCTxReceipt, err error) {
	txStatus, err := b.GetTransactionStatus(swapInfo.Hash)
	if err != nil {
		log.Error("get tx receipt failed", "hash", swapInfo.Hash, "err", err)
		return nil, err
	}
	if txStatus == nil || txStatus.BlockHeight == 0 {
		return nil, tokens.ErrTxNotFound
	}
	if txStatus.BlockHeight < b.ChainConfig.InitialHeight {
		return nil, tokens.ErrTxBeforeInitialHeight
	}

	swapInfo.Height = txStatus.BlockHeight  // Height
	swapInfo.Timestamp = txStatus.BlockTime // Timestamp

	if !allowUnstable && txStatus.Confirmations < b.ChainConfig.Confirmations {
		return nil, tokens.ErrTxNotStable
	}

	receipt, ok := txStatus.Receipt.(*types.RPCTxReceipt)
	if !ok || !receipt.IsStatusOk() {
		return receipt, tokens.ErrTxWithWrongReceipt
	}

	return receipt, nil
}

func (b *Bridge) verifySwapTxReceipt(swapInfo *tokens.SwapTxInfo, receipt *types.RPCTxReceipt) error {
	if receipt.Recipient == nil {
		return tokens.ErrTxWithWrongContract
	}

	swapInfo.TxTo = receipt.Recipient.LowerHex() // TxTo
	swapInfo.From = receipt.From.LowerHex()      // From

	if !params.AllowCallByContract() &&
		!common.IsEqualIgnoreCase(swapInfo.TxTo, b.ChainConfig.RouterContract) &&
		!params.IsInCallByContractWhitelist(b.ChainConfig.ChainID, swapInfo.TxTo) {
		return tokens.ErrTxWithWrongContract
	}

	return nil
}

func (b *Bridge) verifyERC20SwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	swapInfo.To = rlog.Address.LowerHex() // To
	if !common.IsEqualIgnoreCase(rlog.Address.LowerHex(), b.ChainConfig.RouterContract) {
		return tokens.ErrTxWithWrongContract
	}

	logTopic := rlog.Topics[0].Bytes()
	switch {
	case bytes.Equal(logTopic, LogAnySwapOutTopic):
		err = b.parseERC20SwapoutTxLog(swapInfo, rlog)
	case bytes.Equal(logTopic, LogAnySwapTradeTokensForTokensTopic):
		err = b.parseERC20SwapTradeTxLog(swapInfo, rlog, false)
	case bytes.Equal(logTopic, LogAnySwapTradeTokensForNativeTopic):
		err = b.parseERC20SwapTradeTxLog(swapInfo, rlog, true)
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

	return nil
}

func (b *Bridge) parseERC20SwapoutTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) error {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) != 96 {
		return abicoder.ErrParseDataError
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	erc20SwapInfo.Token = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.Bind = common.BytesToAddress(logTopics[3].Bytes()).LowerHex()
	swapInfo.Value = common.GetBigInt(logData, 0, 32)
	if params.IsUseFromChainIDInReceiptDisabled(b.ChainConfig.ChainID) {
		swapInfo.FromChainID = b.ChainConfig.GetChainID()
	} else {
		swapInfo.FromChainID = common.GetBigInt(logData, 32, 32)
	}
	swapInfo.ToChainID = common.GetBigInt(logData, 64, 32)

	tokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	erc20SwapInfo.TokenID = tokenCfg.TokenID

	return nil
}

func (b *Bridge) parseERC20SwapTradeTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog, forNative bool) error {
	if !params.IsSwapTradeEnabled() {
		return tokens.ErrSwapTradeNotSupport
	}
	logTopics := rlog.Topics
	if len(logTopics) != 3 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) < 192 {
		return abicoder.ErrParseDataError
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	erc20SwapInfo.ForNative = forNative
	swapInfo.From = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.Bind = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	path, err := abicoder.ParseAddressSliceInData(logData, 0)
	if err != nil {
		return err
	}
	if len(path) < 3 {
		return tokens.ErrTxWithWrongPath
	}
	swapInfo.Value = common.GetBigInt(logData, 32, 32)
	erc20SwapInfo.AmountOutMin = common.GetBigInt(logData, 64, 32)
	if params.IsUseFromChainIDInReceiptDisabled(b.ChainConfig.ChainID) {
		swapInfo.FromChainID = b.ChainConfig.GetChainID()
	} else {
		swapInfo.FromChainID = common.GetBigInt(logData, 96, 32)
	}
	swapInfo.ToChainID = common.GetBigInt(logData, 128, 32)

	erc20SwapInfo.Token = path[0]
	erc20SwapInfo.Path = path[1:]

	tokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	erc20SwapInfo.TokenID = tokenCfg.TokenID

	return checkSwapTradePath(swapInfo)
}

// amend trade path [0] if missing,
// then check path exists in pairs of dest chain
func checkSwapTradePath(swapInfo *tokens.SwapTxInfo) error {
	dstChainID := swapInfo.ToChainID.String()
	dstBridge := router.GetBridgeByChainID(dstChainID)
	if dstBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, dstChainID)
	if multichainToken == "" {
		return tokens.ErrMissTokenConfig
	}
	tokenCfg := dstBridge.GetTokenConfig(multichainToken)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	path := erc20SwapInfo.Path
	if len(path) < 2 {
		return tokens.ErrTxWithWrongPath
	}
	srcToken := common.HexToAddress(path[0])
	if !(srcToken == tokenCfg.GetUnderlying() || srcToken == common.HexToAddress(multichainToken)) {
		log.Warn("check swap trade path first element failed", "token", path[0])
		return tokens.ErrTxWithWrongPath
	}
	if erc20SwapInfo.ForNative {
		wNative := dstBridge.GetChainConfig().GetRouterWNative()
		wNativeAddr := common.HexToAddress(wNative)
		if wNativeAddr == (common.Address{}) {
			return tokens.ErrSwapTradeNotSupport
		}
		if wNativeAddr != common.HexToAddress(path[len(path)-1]) {
			log.Warn("check swap trade path last element failed", "token", path[len(path)-1])
			return tokens.ErrTxWithWrongPath
		}
	}
	factory := dstBridge.GetChainConfig().GetRouterFactory()
	if common.HexToAddress(factory) == (common.Address{}) {
		return tokens.ErrSwapTradeNotSupport
	}

	swapTrader, ok := dstBridge.(tokens.ISwapTrade)
	if !ok {
		return tokens.ErrSwapTradeNotSupport
	}

	for i := 1; i < len(path); i++ {
		pairs, err := swapTrader.GetPairFor(factory, path[i-1], path[i])
		if err != nil || pairs == "" {
			if errors.Is(err, tokens.ErrRPCQueryError) {
				return err
			}
			log.Warn("check swap trade path pairs failed", "factory", factory, "token0", path[i-1], "token1", path[i], "err", err)
			return tokens.ErrTxWithWrongPath
		}
	}
	return nil
}

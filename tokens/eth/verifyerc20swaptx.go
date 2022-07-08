package eth

import (
	"bytes"
	"fmt"
	"math/big"
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
	// LogAnySwapOut(address token, address from, string to, uint amount, uint fromChainID, uint toChainID);
	LogAnySwapOut2Topic = common.FromHex("0x409e0ad946b19f77602d6cf11d59e1796ddaa4828159a0b4fb7fa2ff6b161b79")
	// LogAnySwapOutAndCall(address token, address from, string to, uint amount, uint fromChainID, uint toChainID, string anycallProxy, bytes data);
	LogAnySwapOutAndCallTopic = common.FromHex("0x8e7e5695fff09074d4c7d6c71615fd382427677f75f460c522357233f3bd3ec3")
	// LogAnySwapTradeTokensForTokens(address[] path, address from, address to, uint amountIn, uint amountOutMin, uint fromChainID, uint toChainID);
	LogAnySwapTradeTokensForTokensTopic = common.FromHex("0xfea6abdf4fd32f20966dff7619354cd82cd43dc78a3bee479f04c74dbfc585b3")
	// LogAnySwapTradeTokensForNative(address[] path, address from, address to, uint amountIn, uint amountOutMin, uint fromChainID, uint toChainID);
	LogAnySwapTradeTokensForNativeTopic = common.FromHex("0x278277e0209c347189add7bd92411973b5f6b8644f7ac62ea1be984ce993f8f4")

	anySwapOutUnderlyingWithPermitFuncHash         = common.FromHex("0x8d7d3eea")
	anySwapOutUnderlyingWithTransferPermitFuncHash = common.FromHex("0x1b91a934")
)

func (b *Bridge) verifyERC20SwapTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType // SwapType
	swapInfo.Hash = strings.ToLower(txHash)  // Hash
	swapInfo.LogIndex = logIndex             // LogIndex

	receipt, err := b.getSwapTxReceipt(swapInfo, allowUnstable)
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

	err = b.checkTokenReceived(swapInfo, receipt)
	if err != nil {
		return swapInfo, err
	}

	err = b.checkERC20SwapInfo(swapInfo)
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
			"token", swapInfo.ERC20SwapInfo.Token, "tokenID", swapInfo.ERC20SwapInfo.TokenID,
		}
		if len(swapInfo.ERC20SwapInfo.Path) > 0 {
			ctx = append(ctx,
				"forNative", swapInfo.ERC20SwapInfo.ForNative,
				"forUnderlying", swapInfo.ERC20SwapInfo.ForUnderlying,
				"amountOutMin", swapInfo.ERC20SwapInfo.AmountOutMin,
			)
		} else if swapInfo.ERC20SwapInfo.CallProxy != "" {
			ctx = append(ctx,
				"callProxy", swapInfo.ERC20SwapInfo.CallProxy,
			)
		}
		log.Info("verify router swap tx stable pass", ctx...)
	}

	return swapInfo, nil
}

func (b *Bridge) checkERC20SwapInfo(swapInfo *tokens.SwapTxInfo) error {
	err := b.checkCallByContract(swapInfo)
	if err != nil {
		return err
	}
	err = b.checkSwapWithPermit(swapInfo)
	if err != nil {
		return err
	}

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
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", swapInfo.ToChainID, "txid", swapInfo.Hash)
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
	if erc20SwapInfo.ForUnderlying && common.HexToAddress(toTokenCfg.GetUnderlying()) == (common.Address{}) {
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

	if receipt.Recipient == nil {
		if !params.AllowCallByConstructor() {
			return nil, tokens.ErrTxWithWrongContract
		}
	} else {
		swapInfo.TxTo = receipt.Recipient.LowerHex() // TxTo
	}
	swapInfo.From = receipt.From.LowerHex() // From
	if *receipt.From == (common.Address{}) {
		return nil, tokens.ErrTxWithWrongSender
	}

	return receipt, nil
}

func (b *Bridge) checkCallByContract(swapInfo *tokens.SwapTxInfo) error {
	txTo := swapInfo.TxTo
	routerContract := b.GetRouterContract(swapInfo.GetToken())
	if routerContract == "" {
		return tokens.ErrMissRouterInfo
	}

	if !params.AllowCallByContract() &&
		!common.IsEqualIgnoreCase(txTo, routerContract) &&
		!params.IsInCallByContractWhitelist(b.ChainConfig.ChainID, txTo) {
		if params.CheckEIP1167Master() {
			master := b.GetEIP1167Master(common.HexToAddress(txTo))
			if master != (common.Address{}) &&
				params.IsInCallByContractWhitelist(b.ChainConfig.ChainID, master.LowerHex()) {
				return nil
			}
		}
		if params.HasCallByContractCodeHashWhitelist(b.ChainConfig.ChainID) {
			codehash := b.GetContractCodeHash(common.HexToAddress(txTo))
			if codehash != (common.Hash{}) &&
				params.IsInCallByContractCodeHashWhitelist(b.ChainConfig.ChainID, codehash.String()) {
				return nil
			}
		}
		log.Warn("tx to with wrong contract", "txTo", txTo, "want", routerContract)
		return tokens.ErrTxWithWrongContract
	}

	return nil
}

func (b *Bridge) verifyERC20SwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	swapInfo.To = rlog.Address.LowerHex() // To

	logTopic := rlog.Topics[0].Bytes()
	switch {
	case bytes.Equal(logTopic, LogAnySwapOutTopic):
		err = b.parseERC20SwapoutTxLog(swapInfo, rlog)
	case bytes.Equal(logTopic, LogAnySwapOut2Topic):
		err = b.parseERC20Swapout2TxLog(swapInfo, rlog)
	case bytes.Equal(logTopic, LogAnySwapOutAndCallTopic):
		err = b.parseERC20SwapoutAndCallTxLog(swapInfo, rlog)
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

	routerContract := b.GetRouterContract(swapInfo.ERC20SwapInfo.Token)
	if routerContract == "" {
		return tokens.ErrMissRouterInfo
	}
	if !common.IsEqualIgnoreCase(rlog.Address.LowerHex(), routerContract) {
		log.Warn("router contract mismatch", "have", rlog.Address.LowerHex(), "want", routerContract)
		return tokens.ErrTxWithWrongContract
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

func (b *Bridge) parseERC20Swapout2TxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 3 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) < 160 {
		return abicoder.ErrParseDataError
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	erc20SwapInfo.Token = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
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
	swapInfo.ToChainID = common.GetBigInt(logData, 96, 32)

	tokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	erc20SwapInfo.TokenID = tokenCfg.TokenID

	return nil
}

func (b *Bridge) parseERC20SwapoutAndCallTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 3 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if len(logData) < 288 {
		return abicoder.ErrParseDataError
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	erc20SwapInfo.Token = common.BytesToAddress(logTopics[1].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
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
	swapInfo.ToChainID = common.GetBigInt(logData, 96, 32)

	erc20SwapInfo.CallProxy, err = abicoder.ParseStringInData(logData, 128)
	if err != nil {
		return err
	}
	erc20SwapInfo.CallData, err = abicoder.ParseBytesInData(logData, 160)
	if err != nil {
		return err
	}

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
//nolint:gocyclo // allow long check trade path
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
	if !(srcToken == common.HexToAddress(tokenCfg.GetUnderlying()) ||
		srcToken == common.HexToAddress(multichainToken)) {
		log.Warn("check swap trade path first element failed", "token", path[0])
		return tokens.ErrTxWithWrongPath
	}
	routerContract := dstBridge.GetRouterContract(multichainToken)
	if routerContract == "" {
		return tokens.ErrMissRouterInfo
	}
	routerInfo := router.GetRouterInfo(routerContract, dstChainID)
	if routerInfo == nil {
		return tokens.ErrMissRouterInfo
	}
	if erc20SwapInfo.ForNative {
		wNative := routerInfo.RouterWNative
		wNativeAddr := common.HexToAddress(wNative)
		if wNativeAddr == (common.Address{}) {
			return tokens.ErrSwapTradeNotSupport
		}
		if wNativeAddr != common.HexToAddress(path[len(path)-1]) {
			log.Warn("check swap trade path last element failed", "token", path[len(path)-1])
			return tokens.ErrTxWithWrongPath
		}
	}
	factory := routerInfo.RouterFactory
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
			if tokens.IsRPCQueryOrNotFoundError(err) {
				return err
			}
			log.Warn("check swap trade path pairs failed", "factory", factory, "token0", path[i-1], "token1", path[i], "err", err)
			return tokens.ErrTxWithWrongPath
		}
	}
	return nil
}

func (b *Bridge) checkSwapWithPermit(swapInfo *tokens.SwapTxInfo) error {
	if params.IsSwapWithPermitEnabled() {
		return nil
	}
	if swapInfo.ERC20SwapInfo.CallProxy != "" {
		return nil
	}
	routerContract := b.GetRouterContract(swapInfo.ERC20SwapInfo.Token)
	if routerContract == "" {
		return tokens.ErrMissRouterInfo
	}

	if common.IsEqualIgnoreCase(swapInfo.TxTo, routerContract) {
		tx, err := b.GetTransactionByHash(swapInfo.Hash)
		if err != nil {
			return err
		}
		if tx.Payload == nil || len(*tx.Payload) < 4 {
			return tokens.ErrUnsupportedFuncHash
		}

		data := *tx.Payload
		funcHash := data[:4]
		if bytes.Equal(funcHash, anySwapOutUnderlyingWithPermitFuncHash) ||
			bytes.Equal(funcHash, anySwapOutUnderlyingWithTransferPermitFuncHash) {
			return tokens.ErrUnsupportedFuncHash
		}
		return nil
	}

	return nil
}

// check underlying token is really received (or burned)
// or anyToken is really burned by anySwapOut
// anySwapOut burn anyToken from sender to zero address
// anySwapOutUnderlying transfer underlying from sender to anyToken
// anySwapOutNative transfer underlying from router to anyToken
//nolint:funlen,gocyclo // allow long method
func (b *Bridge) checkTokenReceived(swapInfo *tokens.SwapTxInfo, receipt *types.RPCTxReceipt) error {
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	token := erc20SwapInfo.Token
	tokenID := erc20SwapInfo.TokenID
	tokenCfg := b.GetTokenConfig(token)
	if tokenCfg == nil || tokenID == "" {
		return tokens.ErrMissTokenConfig
	}
	if params.DontCheckTokenReceived(tokenID) {
		return nil
	}
	tokenAddr := common.HexToAddress(token)
	underlyingAddr := tokenCfg.GetUnderlying()
	if common.HexToAddress(underlyingAddr) == (common.Address{}) {
		return nil
	}
	routerContract := b.GetRouterContract(token)
	if routerContract == "" {
		return tokens.ErrMissRouterInfo
	}
	swapFromAddr := common.HexToAddress(swapInfo.From)

	log.Info("start check token received",
		"token", token, "tokenID", tokenID, "logIndex", swapInfo.LogIndex,
		"underlying", underlyingAddr, "router", routerContract,
		"swapFrom", swapInfo.From, "swapValue", swapInfo.Value, "swapID", swapInfo.Hash)

	transferTopic := erc20CodeParts["LogTransfer"]

	var recvAmount *big.Int
	var isBurn bool
	// find in reverse order
	for i := swapInfo.LogIndex - 1; i >= 0; i-- {
		rlog := receipt.Logs[i]
		if common.IsEqualIgnoreCase(rlog.Address.LowerHex(), routerContract) {
			log.Info("check token received prevent reentrance", "index", i, "logAddress", rlog.Address.LowerHex(), "logTopic", rlog.Topics[0].Hex(), "swapID", swapInfo.Hash)
			break // prevent re-entrance
		}
		if rlog.Removed != nil && *rlog.Removed {
			log.Info("check token received ignore removed log", "index", i, "swapID", swapInfo.Hash)
			continue
		}
		if len(rlog.Topics) != 3 || rlog.Data == nil {
			continue
		}
		if !bytes.Equal(rlog.Topics[0][:], transferTopic) {
			continue
		}
		from := common.BytesToAddress(rlog.Topics[1][:]).LowerHex()
		toAddr := common.BytesToAddress(rlog.Topics[2][:])
		isBurn = toAddr == (common.Address{})

		log.Info("check token received found transfer log", "index", i, "logAddress", rlog.Address.LowerHex(), "from", from, "to", toAddr.LowerHex(), "swapID", swapInfo.Hash)

		if *rlog.Address == common.HexToAddress(underlyingAddr) {
			if isBurn {
				if common.IsEqualIgnoreCase(from, swapInfo.From) {
					recvAmount = common.GetBigInt(*rlog.Data, 0, 32)
				}
				log.Info("check token received found underlying.burn", "index", i, "amount", recvAmount, "from", from, "swapID", swapInfo.Hash)
				break
			} else if toAddr == tokenAddr {
				if common.IsEqualIgnoreCase(from, swapInfo.From) ||
					common.IsEqualIgnoreCase(from, routerContract) {
					recvAmount = common.GetBigInt(*rlog.Data, 0, 32)
				}
				log.Info("check token received found underlying.transfer", "index", i, "amount", recvAmount, "from", from, "swapID", swapInfo.Hash)
				break
			}
			log.Warn("check token received unexpected underlying transfer", "index", i, "from", from, "to", toAddr.LowerHex(), "swapID", swapInfo.Hash)
		} else if *rlog.Address == tokenAddr {
			// anySwapout token with underlying, but calling anyToken.burn
			if !isBurn {
				continue
			}
			if !common.IsEqualIgnoreCase(from, swapInfo.From) {
				log.Info("check token received ingore mismatched burner", "index", i, "swapID", swapInfo.Hash)
				continue
			}
			if i >= 2 {
				pLog := receipt.Logs[i-1]
				// if the prvious log is token mint, ignore this log
				// v5 and previous router mode
				if *pLog.Address == tokenAddr &&
					bytes.Equal(pLog.Topics[0][:], transferTopic) &&
					common.BytesToAddress(pLog.Topics[1][:]) == (common.Address{}) &&
					common.BytesToAddress(pLog.Topics[2][:]) == swapFromAddr {
					log.Info("check token received ingore anytoken mint and burn", "index", i, "swapID", swapInfo.Hash)
					i--
					continue
				}
			}
			recvAmount = common.GetBigInt(*rlog.Data, 0, 32)
			log.Info("check token received found anyToken.burn", "index", i, "amount", recvAmount, "swapID", swapInfo.Hash)
			break
		}
	}
	if recvAmount == nil {
		log.Warn("check token received found none", "swapID", swapInfo.Hash)
		return fmt.Errorf("no underlying token received")
	}
	// at least receive 80% (consider fees and deflation burning)
	minRecvAmount := new(big.Int).Mul(swapInfo.Value, big.NewInt(4))
	minRecvAmount.Div(minRecvAmount, big.NewInt(5))
	if recvAmount.Cmp(minRecvAmount) < 0 {
		log.Warn("check token received failed", "isBurn", isBurn, "received", recvAmount, "swapValue", swapInfo.Value, "minRecvAmount", minRecvAmount, "swapID", swapInfo.Hash)
		return fmt.Errorf("check underlying token received failed")
	}
	log.Info("check token received success", "isBurn", isBurn, "received", recvAmount, "swapValue", swapInfo.Value, "swapID", swapInfo.Hash)
	return nil
}

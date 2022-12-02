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
	// LogAnySwapOutMixPool(address token, address from, string to, uint256 amount, uint256 fromChainID, uint256 toChainID)
	LogAnySwapOutMixPoolTopic = common.FromHex("0xb89f5e0fee14552a1edac7c525bc18eb2c5fad996def47de882b82fdea286fd7")
	// LogAnySwapOut(bytes32 swapoutID, address token, address from, string receiver, uint256 amount, uint256 toChainID)
	LogAnySwapOutV7Topic = common.FromHex("0x0d969ae475ff6fcaf0dcfa760d4d8607244e8d95e9bf426f8d5d69f9a3e525af")
	// LogAnySwapOutAndCall(bytes32 swapoutID, address token, address from, string receiver, uint256 amount, uint256 toChainID, string anycallProxy, bytes data)
	LogAnySwapOutAndCallV7Topic = common.FromHex("0x968608314ec29f6fd1a9f6ef9e96247a4da1a683917569706e2d2b60ca7c0a6d")

	anySwapOutUnderlyingWithPermitFuncHash         = common.FromHex("0x8d7d3eea")
	anySwapOutUnderlyingWithTransferPermitFuncHash = common.FromHex("0x1b91a934")

	// PauseSwapIntoTokenVersion pause swap into a dest token which is configed with this version
	// usage: when we replace an old token with a new one,
	// but the old token has some swapouts not processed,
	// then we can replace temply to the old token and with this special version,
	// after old token swapouts are processed, we should replace back to the new token config.
	PauseSwapIntoTokenVersion = uint64(90000)

	LogTokenTransferTopics = []common.Hash{
		// Transfer(address,address,uint256)
		common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
	}
)

func (b *Bridge) VerifyERC20SwapTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
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

	err = b.checkTokenBalance(swapInfo, receipt)
	if err != nil {
		return swapInfo, err
	}

	if params.IsSwapoutForbidden(b.ChainConfig.ChainID, swapInfo.ERC20SwapInfo.TokenID) {
		return swapInfo, tokens.ErrSwapoutForbidden
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
		if swapInfo.ERC20SwapInfo.CallProxy != "" {
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
	if swapInfo.FromChainID.Cmp(swapInfo.ToChainID) == 0 {
		return tokens.ErrSameFromAndToChainID
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
	if toTokenCfg.ContractVersion == PauseSwapIntoTokenVersion {
		return tokens.ErrPauseSwapInto
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
		key := fmt.Sprintf("%v:%v:%v", b.ChainConfig.ChainID, swapInfo.Hash, swapInfo.LogIndex)
		flag := params.GetSpecialFlag(key)
		if !strings.EqualFold(flag, "PassCheckInitialHeight") {
			return nil, tokens.ErrTxBeforeInitialHeight
		}
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
			log.Warn("disallow constructor tx", "chainID", b.ChainConfig.ChainID, "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", tokens.ErrTxWithWrongContract)
			return nil, tokens.ErrTxWithWrongContract
		}
	} else {
		swapInfo.TxTo = receipt.Recipient.LowerHex() // TxTo
	}
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
		log.Warn("tx to address mismatch", "have", txTo, "want", routerContract, "chainID", b.ChainConfig.ChainID, "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", tokens.ErrTxWithWrongContract)
		return tokens.ErrTxWithWrongContract
	}

	return nil
}

func (b *Bridge) verifyERC20SwapTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
	if rlog == nil || len(rlog.Topics) == 0 {
		return tokens.ErrSwapoutLogNotFound
	}

	swapInfo.To = rlog.Address.LowerHex() // To

	logTopic := rlog.Topics[0].Bytes()
	switch {
	case bytes.Equal(logTopic, LogAnySwapOutTopic):
		err = b.parseERC20SwapoutTxLog(swapInfo, rlog)
	case bytes.Equal(logTopic, LogAnySwapOut2Topic):
		err = b.parseERC20Swapout2TxLog(swapInfo, rlog)
	case bytes.Equal(logTopic, LogAnySwapOutV7Topic):
		err = b.parseERC20SwapoutV7TxLog(swapInfo, rlog, false)
	case bytes.Equal(logTopic, LogAnySwapOutAndCallV7Topic):
		err = b.parseERC20SwapoutV7TxLog(swapInfo, rlog, true)
	case bytes.Equal(logTopic, LogAnySwapOutMixPoolTopic):
		swapInfo.SwapType = tokens.ERC20SwapTypeMixPool // update SwapType
		err = b.parseERC20SwapoutMixPoolTxLog(swapInfo, rlog)
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
		log.Warn("tx to address mismatch", "have", rlog.Address.LowerHex(), "want", routerContract, "chainID", b.ChainConfig.ChainID, "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "err", tokens.ErrTxWithWrongContract)
		return tokens.ErrTxWithWrongContract
	}

	swapoutID := swapInfo.ERC20SwapInfo.SwapoutID
	if swapoutID != "" {
		routerInfo := router.GetRouterInfo(routerContract, b.ChainConfig.ChainID)
		if routerInfo == nil {
			return tokens.ErrMissRouterInfo
		}
		exist, err := b.IsSwapoutIDExist(routerInfo.RouterSecurity, swapoutID)
		if err != nil {
			return err
		}
		if !exist {
			return tokens.ErrSwapoutIDNotExist
		}
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

func (b *Bridge) parseERC20SwapoutMixPoolTxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog) (err error) {
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

func (b *Bridge) parseERC20SwapoutV7TxLog(swapInfo *tokens.SwapTxInfo, rlog *types.RPCLog, withCall bool) (err error) {
	logTopics := rlog.Topics
	if len(logTopics) != 4 {
		return tokens.ErrTxWithWrongTopics
	}
	logData := *rlog.Data
	if (!withCall && len(logData) < 128) || (withCall && len(logData) < 256) {
		return abicoder.ErrParseDataError
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	erc20SwapInfo.SwapoutID = common.BytesToHash(logTopics[1].Bytes()).Hex()
	erc20SwapInfo.Token = common.BytesToAddress(logTopics[2].Bytes()).LowerHex()
	swapInfo.From = common.BytesToAddress(logTopics[3].Bytes()).LowerHex()
	swapInfo.Bind, err = abicoder.ParseStringInData(logData, 0)
	if err != nil {
		return err
	}
	swapInfo.Value = common.GetBigInt(logData, 32, 32)
	swapInfo.FromChainID = b.ChainConfig.GetChainID()
	swapInfo.ToChainID = common.GetBigInt(logData, 64, 32)

	if withCall {
		erc20SwapInfo.CallProxy, err = abicoder.ParseStringInData(logData, 96)
		if err != nil {
			return err
		}
		erc20SwapInfo.CallData, err = abicoder.ParseBytesInData(logData, 128)
		if err != nil {
			return err
		}
	}

	tokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	erc20SwapInfo.TokenID = tokenCfg.TokenID

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
		tx, err := b.EvmContractBridge.GetTransactionByHash(swapInfo.Hash)
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
	if common.HexToAddress(underlyingAddr) == (common.Address{}) ||
		tokenCfg.IsWrapperTokenVersion() {
		return nil
	}
	routerContract := b.GetRouterContract(token)
	if routerContract == "" {
		return tokens.ErrMissRouterInfo
	}
	swapFromAddr := common.HexToAddress(swapInfo.From)

	log.Info("start check token received", "chainID", b.ChainConfig.ChainID,
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
			log.Info("check token received prevent reentrance", "chainID", b.ChainConfig.ChainID, "index", i, "logAddress", rlog.Address.LowerHex(), "logTopic", rlog.Topics[0].Hex(), "swapID", swapInfo.Hash)
			break // prevent re-entrance
		}
		if rlog.Removed != nil && *rlog.Removed {
			log.Info("check token received ignore removed log", "chainID", b.ChainConfig.ChainID, "index", i, "swapID", swapInfo.Hash)
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

		log.Info("check token received found transfer log", "chainID", b.ChainConfig.ChainID, "index", i, "logAddress", rlog.Address.LowerHex(), "from", from, "to", toAddr.LowerHex(), "swapID", swapInfo.Hash)

		if *rlog.Address == common.HexToAddress(underlyingAddr) {
			if isBurn {
				if common.IsEqualIgnoreCase(from, swapInfo.From) {
					recvAmount = common.GetBigInt(*rlog.Data, 0, 32)
				}
				log.Info("check token received found underlying.burn", "chainID", b.ChainConfig.ChainID, "index", i, "amount", recvAmount, "from", from, "swapID", swapInfo.Hash)
				break
			} else if toAddr == tokenAddr {
				if common.IsEqualIgnoreCase(from, swapInfo.From) ||
					common.IsEqualIgnoreCase(from, routerContract) {
					recvAmount = common.GetBigInt(*rlog.Data, 0, 32)
				}
				log.Info("check token received found underlying.transfer", "chainID", b.ChainConfig.ChainID, "index", i, "amount", recvAmount, "from", from, "swapID", swapInfo.Hash)
				break
			}
			log.Warn("check token received unexpected underlying transfer", "chainID", b.ChainConfig.ChainID, "index", i, "from", from, "to", toAddr.LowerHex(), "swapID", swapInfo.Hash)
		} else if *rlog.Address == tokenAddr {
			// anySwapout token with underlying, but calling anyToken.burn
			if !isBurn {
				continue
			}
			if !common.IsEqualIgnoreCase(from, swapInfo.From) {
				log.Info("check token received ingore mismatched burner", "chainID", b.ChainConfig.ChainID, "index", i, "swapID", swapInfo.Hash)
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
					log.Info("check token received ingore anytoken mint and burn", "chainID", b.ChainConfig.ChainID, "index", i, "swapID", swapInfo.Hash)
					i--
					continue
				}
			}
			recvAmount = common.GetBigInt(*rlog.Data, 0, 32)
			log.Info("check token received found anyToken.burn", "chainID", b.ChainConfig.ChainID, "index", i, "amount", recvAmount, "swapID", swapInfo.Hash)
			break
		}
	}
	if recvAmount == nil {
		log.Warn("check token received found none", "chainID", b.ChainConfig.ChainID, "swapID", swapInfo.Hash)
		return fmt.Errorf("%w %v", tokens.ErrVerifyTxUnsafe, "no underlying token received")
	}
	// at least receive 80% (consider fees and deflation burning)
	minRecvAmount := new(big.Int).Mul(swapInfo.Value, big.NewInt(4))
	minRecvAmount.Div(minRecvAmount, big.NewInt(5))
	if recvAmount.Cmp(minRecvAmount) < 0 {
		log.Warn("check token received failed", "chainID", b.ChainConfig.ChainID, "isBurn", isBurn, "received", recvAmount, "swapValue", swapInfo.Value, "minRecvAmount", minRecvAmount, "swapID", swapInfo.Hash)
		return fmt.Errorf("%w %v", tokens.ErrVerifyTxUnsafe, "check underlying token received failed")
	}
	log.Info("check token received success", "chainID", b.ChainConfig.ChainID, "isBurn", isBurn, "received", recvAmount, "swapValue", swapInfo.Value, "swapID", swapInfo.Hash)
	return nil
}

// check token balance updations
func (b *Bridge) checkTokenBalance(swapInfo *tokens.SwapTxInfo, receipt *types.RPCTxReceipt) error {
	if !params.IsCheckTokenBalanceEnabled(b.ChainConfig.ChainID) ||
		params.DontCheckTokenBalance(swapInfo.ERC20SwapInfo.TokenID) {
		return nil
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	token := erc20SwapInfo.Token
	tokenID := erc20SwapInfo.TokenID
	routerContract := b.GetRouterContract(token)

	tokenCfg := b.GetTokenConfig(token)
	if tokenCfg == nil || tokenID == "" {
		return tokens.ErrMissTokenConfig
	}
	underlying := tokenCfg.GetUnderlying()

	blockHeight := receipt.BlockNumber.ToInt().Uint64()

	if swapInfo.LogIndex == 0 {
		return fmt.Errorf("evm erc20 swapout logIndex must be greater than 0")
	}

	// at least receive 80% (consider fees and deflation burning)
	minChangeAmount := new(big.Int).Mul(swapInfo.Value, big.NewInt(4))
	minChangeAmount.Div(minChangeAmount, big.NewInt(5))

	log.Info("start check token balance", "swapValue", swapInfo.Value, "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", tokenID, "chainID", b.ChainConfig.ChainID)

	transferTopic := erc20CodeParts["LogTransfer"]

	var matchedTransfers []*types.RPCLog
	var matchedBurns []*types.RPCLog

	for i := swapInfo.LogIndex - 1; i >= 0; i-- {
		rlog := receipt.Logs[i]
		logAddr := rlog.Address.LowerHex()
		if common.IsEqualIgnoreCase(logAddr, routerContract) {
			break
		}
		if !(common.IsEqualIgnoreCase(logAddr, token) ||
			common.IsEqualIgnoreCase(logAddr, underlying)) {
			continue
		}

		if rlog.Removed != nil && *rlog.Removed {
			continue
		}
		if len(rlog.Topics) != 3 || rlog.Data == nil {
			continue
		}
		if !bytes.Equal(rlog.Topics[0][:], transferTopic) {
			continue
		}

		fromAddr := common.BytesToAddress(rlog.Topics[1][:])
		isMint := fromAddr == (common.Address{})
		if isMint {
			continue
		}
		if !common.IsEqualIgnoreCase(fromAddr.LowerHex(), swapInfo.From) &&
			!common.IsEqualIgnoreCase(fromAddr.LowerHex(), routerContract) {
			continue
		}

		toAddr := common.BytesToAddress(rlog.Topics[2][:])
		isBurn := toAddr == (common.Address{})

		if !(isBurn || common.IsEqualIgnoreCase(toAddr.LowerHex(), token)) {
			continue
		}

		amount := common.GetBigInt(*rlog.Data, 0, 32)
		if amount.Cmp(minChangeAmount) < 0 {
			continue
		}

		if isBurn {
			matchedBurns = append(matchedBurns, rlog)
		} else {
			matchedTransfers = append(matchedTransfers, rlog)
		}
	}

	if len(matchedTransfers) == 0 && len(matchedBurns) == 0 {
		log.Info("check token balance without swapout pattern matched", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "tokenID", tokenID, "chainID", b.ChainConfig.ChainID)
		return fmt.Errorf("no swapout pattern matched")
	}

	log.Info("check token balance with swapout matched", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "tokenID", tokenID, "chainID", b.ChainConfig.ChainID, "matchedTransfers", len(matchedTransfers), "matchedBurns", len(matchedBurns))

	// transfer has priority, and can ignore burn checking when has transfer pattern
	if len(matchedTransfers) > 0 {
		for _, rlog := range matchedTransfers {
			fromAddr := common.BytesToAddress(rlog.Topics[1][:]).LowerHex()
			err := b.checkTokenTransfer(swapInfo, rlog.Address.LowerHex(), fromAddr, token, blockHeight)
			if err != nil {
				log.Warn("check token balance transfer failed", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "tokenID", tokenID, "chainID", b.ChainConfig.ChainID, "tokenAddr", rlog.Address.LowerHex(), "err", err)
				return err
			}
		}
		log.Info("check token balance transfer success", "swapValue", swapInfo.Value, "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "tokenID", tokenID, "chainID", b.ChainConfig.ChainID, "token", token)
	} else {
		for _, rlog := range matchedBurns {
			err := b.checkTokenBurn(swapInfo, rlog.Address.LowerHex(), swapInfo.From, blockHeight)
			if err != nil {
				log.Warn("check token balance burn failed", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "tokenID", tokenID, "chainID", b.ChainConfig.ChainID, "tokenAddr", rlog.Address.LowerHex(), "err", err)
				return err
			}
		}
		log.Info("check token balance burn success", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "tokenID", tokenID, "chainID", b.ChainConfig.ChainID, "token", token)
	}
	return nil
}

func (b *Bridge) checkTotalSupply(swapInfo *tokens.SwapTxInfo, token string, blockHeight uint64, minChangeAmount *big.Int) error {
	prevSupply, err := b.GetErc20TotalSupplyAtHeight(token, fmt.Sprintf("0x%x", blockHeight-1))
	if err != nil {
		log.Info("get prev token total supply failed", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight-1, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "err", err)
		return nil
	}

	postSupply, err := b.GetErc20TotalSupplyAtHeight(token, fmt.Sprintf("0x%x", blockHeight))
	if err != nil {
		log.Info("get post token total supply failed", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "err", err)
		return nil
	}

	if prevSupply.Sign() == 0 || postSupply.Sign() == 0 {
		log.Info("get token total supply returns zero", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID)
		return nil
	}

	actChangeAmount := new(big.Int).Sub(prevSupply, postSupply)
	if actChangeAmount.Cmp(minChangeAmount) < 0 {
		trasferLogs, errf := b.GetContractLogs(common.HexToAddress(token), LogTokenTransferTopics, blockHeight)
		if errf != nil {
			log.Warn("check token total supply get transfer logs failed", "swapValue", swapInfo.Value, "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "err", errf)
		} else if len(trasferLogs) > 0 {
			log.Info("check token total supply get transfer logs success", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "trasferLogs", len(trasferLogs))
			totalAmount := getTokenMintAmount(trasferLogs)
			if new(big.Int).Add(actChangeAmount, totalAmount).Cmp(minChangeAmount) >= 0 {
				return nil
			}
		}

		log.Warn("check token total supply failed", "swapValue", swapInfo.Value, "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "prevSupply", prevSupply, "postSupply", postSupply, "minChangeAmount", minChangeAmount, "actChangeAmount", actChangeAmount, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "trasferLogs", len(trasferLogs))
		return fmt.Errorf("%w %v", tokens.ErrVerifyTxUnsafe, "check total supply failed")
	}

	log.Warn("check token total supply success", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "prevSupply", prevSupply, "postSupply", postSupply, "minChangeAmount", minChangeAmount, "actChangeAmount", actChangeAmount, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID)
	return nil
}

func (b *Bridge) checkAccountBalance(swapInfo *tokens.SwapTxInfo, token, account string, blockHeight uint64, minChangeAmount *big.Int, isDecrease bool) error {
	prevBal, err := b.GetErc20BalanceAtHeight(token, account, fmt.Sprintf("0x%x", blockHeight-1))
	if err != nil {
		log.Info("get prev token balance failed", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight-1, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "account", account, "err", err)
		return nil
	}

	postBal, err := b.GetErc20BalanceAtHeight(token, account, fmt.Sprintf("0x%x", blockHeight))
	if err != nil {
		log.Info("get post token balance failed", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "account", account, "err", err)
		return nil
	}

	if prevBal.Sign() == 0 || postBal.Sign() == 0 {
		log.Info("get token balance returns zero", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "account", account)
		return nil
	}

	var actChangeAmount *big.Int
	if isDecrease {
		actChangeAmount = new(big.Int).Sub(prevBal, postBal)
	} else {
		actChangeAmount = new(big.Int).Sub(postBal, prevBal)
	}

	if actChangeAmount.Cmp(minChangeAmount) < 0 {
		trasferLogs, errf := b.GetContractLogs(common.HexToAddress(token), LogTokenTransferTopics, blockHeight)
		if errf != nil {
			log.Warn("check token balance get transfer logs failed", "swapValue", swapInfo.Value, "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "account", account, "err", errf)
		} else if len(trasferLogs) > 0 {
			log.Warn("check token balance get transfer logs success", "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "trasferLogs", len(trasferLogs), "account", account)
			sendAmount, receiveAmount := getTokenTransferAmount(trasferLogs, account)
			if isDecrease {
				if new(big.Int).Add(actChangeAmount, receiveAmount).Cmp(minChangeAmount) >= 0 {
					return nil
				}
			} else {
				if new(big.Int).Add(actChangeAmount, sendAmount).Cmp(minChangeAmount) >= 0 {
					return nil
				}
			}
		}

		log.Warn("check token balance failed", "swapValue", swapInfo.Value, "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "prevBal", prevBal, "postBal", postBal, "minChangeAmount", minChangeAmount, "actChangeAmount", actChangeAmount, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "trasferLogs", len(trasferLogs), "account", account)
		return fmt.Errorf("%w %v", tokens.ErrVerifyTxUnsafe, "check token balance failed")
	}

	log.Info("check token balance success", "swapValue", swapInfo.Value, "swapID", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "blockHeight", blockHeight, "prevBal", prevBal, "postBal", postBal, "minChangeAmount", minChangeAmount, "actChangeAmount", actChangeAmount, "token", token, "tokenID", swapInfo.ERC20SwapInfo.TokenID, "chainID", b.ChainConfig.ChainID, "account", account)
	return nil
}

// 1. check from's token balance decreased
// 2. check token's total supply decreased
func (b *Bridge) checkTokenBurn(swapInfo *tokens.SwapTxInfo, token, from string, blockHeight uint64) error {
	// at least receive 80% (consider fees and deflation burning)
	minChangeAmount := new(big.Int).Mul(swapInfo.Value, big.NewInt(4))
	minChangeAmount.Div(minChangeAmount, big.NewInt(5))
	err := b.checkAccountBalance(swapInfo, token, from, blockHeight, minChangeAmount, true)
	if err != nil {
		return err
	}
	if !params.DontCheckTokenTotalSupply(swapInfo.ERC20SwapInfo.TokenID) {
		return b.checkTotalSupply(swapInfo, token, blockHeight, minChangeAmount)
	}
	return nil
}

// 1. check from's token balance decreased
// 2. check to's token balance increased
func (b *Bridge) checkTokenTransfer(swapInfo *tokens.SwapTxInfo, token, from, to string, blockHeight uint64) error {
	// at least receive 80% (consider fees and deflation burning)
	minChangeAmount := new(big.Int).Mul(swapInfo.Value, big.NewInt(4))
	minChangeAmount.Div(minChangeAmount, big.NewInt(5))
	routerContract := b.GetRouterContract(token)
	if !common.IsEqualIgnoreCase(from, routerContract) {
		err := b.checkAccountBalance(swapInfo, token, from, blockHeight, minChangeAmount, true)
		if err != nil {
			return err
		}
	}
	return b.checkAccountBalance(swapInfo, token, to, blockHeight, minChangeAmount, false)
}

func getTokenTransferAmount(trasferLogs []*types.RPCLog, account string) (sendAmount, receiveAmount *big.Int) {
	sendAmount = big.NewInt(0)
	receiveAmount = big.NewInt(0)
	for _, rlog := range trasferLogs {
		if len(rlog.Topics) != 3 || rlog.Data == nil || len(*rlog.Data) < 32 {
			log.Error("get logs return wrong result", "topics", rlog.Topics, "data", rlog.Data)
			continue
		}
		amount := common.GetBigInt(*rlog.Data, 0, 32)
		sender := common.BytesToAddress(rlog.Topics[1][:]).LowerHex()
		receiver := common.BytesToAddress(rlog.Topics[2][:]).LowerHex()
		if common.IsEqualIgnoreCase(sender, account) {
			sendAmount.Add(sendAmount, amount)
		}
		if common.IsEqualIgnoreCase(receiver, account) {
			receiveAmount.Add(receiveAmount, amount)
		}
	}
	return sendAmount, receiveAmount
}

func getTokenMintAmount(trasferLogs []*types.RPCLog) *big.Int {
	totalAmount := big.NewInt(0)
	for _, rlog := range trasferLogs {
		if len(rlog.Topics) != 3 || rlog.Data == nil || len(*rlog.Data) < 32 {
			log.Error("get logs return wrong result", "topics", rlog.Topics, "data", rlog.Data)
			continue
		}
		fromAddr := common.BytesToAddress(rlog.Topics[1][:])
		isMint := fromAddr == (common.Address{})
		if !isMint {
			continue
		}
		logData := *rlog.Data
		amount := common.GetBigInt(logData, 0, 32)
		totalAmount.Add(totalAmount, amount)
	}
	return totalAmount
}

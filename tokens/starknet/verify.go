package starknet

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet/rpcv02"
	"github.com/dontpanicdao/caigo/types"
)

const anySwapOutSelector = "0x1835440bee9143eda55679e8067e9003ec48f85ca70a484ace777b68a78ce8a"

func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if len(msgHashes) < 1 {
		return tokens.ErrMsgHash
	}
	tx, ok := rawTx.(FunctionCallWithDetails)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	hash, err := b.TransactionHash(tx.Call, tx.Nonce, tx.MaxFee)
	if err != nil {
		return err
	}
	if !strings.EqualFold(hash.String(), msgHashes[0]) {
		return fmt.Errorf("msg hash not match, recover: %v, claiming: %v", hash.String(), msgHashes[0])
	}
	return nil
}

func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifySwapoutTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

func (b *Bridge) verifySwapoutTx(txHash string, LogIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = LogIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	rawReceipt, err := b.provider.TransactionReceipt(txHash)
	if err != nil {
		return swapInfo, tokens.ErrTxWithWrongReceipt
	}
	receipt, ok := rawReceipt.(rpcv02.InvokeTransactionReceipt)
	if !ok {
		return swapInfo, tokens.ErrInvalidInvokeReceipt
	}

	if !allowUnstable {
		txHeight := receipt.BlockNumber
		if txHeight < b.ChainConfig.InitialHeight {
			return swapInfo, tokens.ErrTxBeforeInitialHeight
		}

		txStatus := string(receipt.Status)
		if !strings.EqualFold(txStatus, string(types.TransactionAcceptedOnL2)) && !strings.EqualFold(txStatus, string(types.TransactionAcceptedOnL1)) {
			return swapInfo, tokens.ErrTxWithWrongStatus
		}
	}

	err = b.verifySwapoutEvents(swapInfo, receipt)
	if err != nil {
		return swapInfo, err
	}

	err = b.checkSwapoutInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify router swap tx stable pass", "identifier", params.GetIdentifier(),
			"from", swapInfo.From, "to", swapInfo.To, "bind", swapInfo.Bind, "value", swapInfo.Value,
			"txid", txHash, "logIndex", swapInfo.LogIndex, "height", swapInfo.Height, "timestamp", swapInfo.Timestamp,
			"fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID,
			"token", swapInfo.ERC20SwapInfo.Token,
			"tokenID", swapInfo.ERC20SwapInfo.TokenID)
	}

	return swapInfo, nil
}

func (b *Bridge) verifySwapoutEvents(swapInfo *tokens.SwapTxInfo, receipt rpcv02.InvokeTransactionReceipt) error {
	// filter event
	var logIndex int
	var isSwapOutLogExist bool
	for i, event := range receipt.Events {
		if len(event.Keys) > 0 && common.IsEqualIgnoreCase(event.Keys[0], anySwapOutSelector) {
			isSwapOutLogExist = true
			logIndex = i
			swapInfo.LogIndex = logIndex
			break
		}
	}

	if !isSwapOutLogExist {
		return tokens.ErrSwapoutLogNotFound
	}

	event := &receipt.Events[logIndex]

	routerContract := b.ChainConfig.RouterContract

	swapInfo.TxTo = event.FromAddress.String()

	if !params.AllowCallByContract() &&
		!common.IsEqualIgnoreCase(swapInfo.TxTo, b.ChainConfig.RouterContract) &&
		!params.IsInCallByContractWhitelist(b.ChainConfig.ChainID, swapInfo.TxTo) {
		return tokens.ErrTxWithWrongContract
	}

	swapInfo.To = routerContract
	erc20SwapInfo := &tokens.ERC20SwapInfo{}
	swapInfo.SwapInfo = tokens.SwapInfo{ERC20SwapInfo: erc20SwapInfo}

	//@event
	//func LogAnySwapOut(
	//                   token: felt,
	//                   from_: felt,
	//                   to: felt,
	//                   amount: Uint256,
	//                   cid: felt,
	//                   toChainID: felt) {
	//}
	//TODO: replace log data index number with constants
	erc20SwapInfo.Token = event.Data[0]
	if len(erc20SwapInfo.Token) == 65 {
		removePrefix := event.Data[0][2:]
		erc20SwapInfo.Token = "0x0" + removePrefix
	}
	swapInfo.From = event.Data[1]
	swapInfo.Bind = event.Data[2] // to address
	tokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	erc20SwapInfo.TokenID = tokenCfg.TokenID
	value := types.HexToBN(event.Data[3]).Uint64()
	swapInfo.Value = new(big.Int).SetUint64(value)

	toChainIDHex := event.Data[len(event.Data)-1]
	toChainID := types.HexToBN(toChainIDHex).Uint64()
	swapInfo.ToChainID = new(big.Int).SetUint64(toChainID)
	return nil
}

func (b *Bridge) checkSwapoutInfo(swapInfo *tokens.SwapTxInfo) error {
	if strings.EqualFold(swapInfo.From, swapInfo.To) {
		return tokens.ErrTxWithWrongSender
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

	if !tokens.CheckTokenSwapValue(swapInfo, fromTokenCfg.Decimals, toTokenCfg.Decimals) {
		return tokens.ErrTxWithWrongValue
	}

	bindAddr := swapInfo.Bind
	if !toBridge.IsValidAddress(bindAddr) {
		log.Warn("wrong bind address in swapin", "bind", bindAddr)
		return tokens.ErrWrongBindAddress
	}
	return nil
}

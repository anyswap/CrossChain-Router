package ripple

import (
	"errors"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/websockets"
)

var errTxResultType = errors.New("tx type is not data.TxResult")

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if len(msgHashes) < 1 {
		return fmt.Errorf("must provide msg hash")
	}
	tx, ok := rawTx.(data.Transaction)
	if !ok {
		return fmt.Errorf("ripple tx type error")
	}
	msgHash, msg, err := data.SigningHash(tx)
	if err != nil {
		return fmt.Errorf("rebuild ripple tx msg error, %w", err)
	}
	msg = append(tx.SigningPrefix().Bytes(), msg...)

	pubkey := tx.GetPublicKey().Bytes()
	isEd := isEd25519Pubkey(pubkey)
	var signContent string
	if isEd {
		signContent = common.ToHex(msg)
	} else {
		signContent = msgHash.String()
	}

	if !strings.EqualFold(signContent, msgHashes[0]) {
		return fmt.Errorf("msg hash not match, recover: %v, claiming: %v", signContent, msgHashes[0])
	}

	return nil
}

// VerifyTransaction impl
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

//nolint:gocyclo,funlen // ok
func (b *Bridge) verifySwapoutTx(txHash string, _ int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = 0                             // LogIndex always 0 (do not support multiple in one tx)
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	tx, err := b.GetTransaction(txHash)
	if err != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return swapInfo, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*websockets.TxResult)
	if !ok {
		return swapInfo, errTxResultType
	}

	if !txres.Validated {
		return swapInfo, tokens.ErrTxIsNotValidated
	}

	if !allowUnstable {
		h, errf := b.GetLatestBlockNumber()
		if errf != nil {
			return swapInfo, errf
		}

		txHeight := uint64(txres.TransactionWithMetaData.LedgerSequence)
		swapInfo.Height = txHeight

		if h < txHeight+b.GetChainConfig().Confirmations {
			return swapInfo, tokens.ErrTxNotStable
		}
		if txHeight < b.ChainConfig.InitialHeight {
			return swapInfo, tokens.ErrTxBeforeInitialHeight
		}
	}

	// Check tx status
	if !txres.TransactionWithMetaData.MetaData.TransactionResult.Success() {
		return swapInfo, tokens.ErrTxWithWrongStatus
	}

	asset := txres.TransactionWithMetaData.MetaData.DeliveredAmount.Asset().String()
	token := b.GetTokenConfig(asset)
	if token == nil {
		return swapInfo, tokens.ErrMissTokenConfig
	}

	payment, ok := txres.TransactionWithMetaData.Transaction.(*data.Payment)
	if !ok || payment.GetTransactionType() != data.PAYMENT {
		log.Printf("Not a payment transaction")
		return swapInfo, fmt.Errorf("not a payment transaction")
	}

	txRecipient := payment.Destination.String()
	// special usage, ripple has no router contract, and use deposit methods
	depositAddress := b.GetRouterContract(asset)
	if !common.IsEqualIgnoreCase(txRecipient, depositAddress) {
		return swapInfo, tokens.ErrTxWithWrongReceiver
	}

	erc20SwapInfo := &tokens.ERC20SwapInfo{}
	erc20SwapInfo.Token = asset
	erc20SwapInfo.TokenID = token.TokenID
	swapInfo.SwapInfo = tokens.SwapInfo{ERC20SwapInfo: erc20SwapInfo}

	err = b.checkToken(token, &txres.TransactionWithMetaData)
	if err != nil {
		return swapInfo, err
	}

	if success := parseSwapMemos(swapInfo, payment.Memos); !success {
		log.Info("wrong memos", "memos", common.ToJSONString(payment.Memos, false))
		return swapInfo, tokens.ErrWrongBindAddress
	}

	if !txres.TransactionWithMetaData.MetaData.DeliveredAmount.IsPositive() {
		return swapInfo, tokens.ErrTxWithNoPayment
	}
	amt := tokens.ToBits(txres.TransactionWithMetaData.MetaData.DeliveredAmount.Value.String(), token.Decimals)

	swapInfo.To = depositAddress             // To
	swapInfo.From = payment.Account.String() // From
	swapInfo.Value = amt

	err = b.checkSwapoutInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify swapin pass",
			"asset", asset, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}
	return swapInfo, nil
}

func (b *Bridge) checkToken(token *tokens.TokenConfig, txmeta *data.TransactionWithMetaData) error {
	assetI, exist := assetMap.Load(token.ContractAddress)
	if !exist {
		return fmt.Errorf("non exist asset %v", token.ContractAddress)
	}
	asset := assetI.(*data.Asset)
	if !strings.EqualFold(asset.Currency, txmeta.MetaData.DeliveredAmount.Currency.Machine()) {
		return fmt.Errorf("ripple currency not match")
	}
	if !txmeta.MetaData.DeliveredAmount.Currency.IsNative() {
		if !strings.EqualFold(asset.Issuer, txmeta.MetaData.DeliveredAmount.Issuer.String()) {
			return fmt.Errorf("ripple currency issuer not match")
		}
	} else if !txmeta.MetaData.DeliveredAmount.Issuer.IsZero() {
		return fmt.Errorf("ripple native issuer is not zero")
	}
	return nil
}

func getTargetMemo(memoStr string) (memo string) {
	index := strings.LastIndex(memoStr, "||")
	if index == -1 {
		memo = memoStr
	} else {
		memo = memoStr[index+2:]
	}
	return strings.TrimSpace(memo)
}

func parseSwapMemos(swapInfo *tokens.SwapTxInfo, memos data.Memos) bool {
	for _, memo := range memos {
		memoStr := getTargetMemo(string(memo.Memo.MemoData.Bytes()))
		parts := strings.Split(memoStr, ":")
		if len(parts) < 2 {
			continue
		}
		bindStr := parts[0]
		toChainIDStr := parts[1]
		biToChainID, err := common.GetBigIntFromStr(toChainIDStr)
		if err != nil {
			continue
		}
		dstBridge := router.GetBridgeByChainID(toChainIDStr)
		if dstBridge == nil {
			continue
		}
		if dstBridge.IsValidAddress(bindStr) {
			swapInfo.Bind = bindStr          // Bind
			swapInfo.ToChainID = biToChainID // ToChainID
			return true
		}
	}
	return false
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

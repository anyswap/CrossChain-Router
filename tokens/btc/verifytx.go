package btc

import (
	"errors"
	"regexp"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	errTxResultType = errors.New("tx type is not TransactionResult")
	regexMemo       = regexp.MustCompile(`^OP_RETURN OP_PUSHBYTES_\d* `)
	p2pkhType       = "p2pkh"
	opReturnType    = "op_return"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	return tokens.ErrNotImplemented
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

func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	log.Info("verifySwapoutTx", "txhash", txHash, "logIndex", logIndex, "allowUnstable", allowUnstable)
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	receipts, err := b.getSwapTxReceipt(swapInfo, true)
	if err != nil {
		return swapInfo, err
	}

	if logIndex >= len(receipts) {
		return swapInfo, tokens.ErrLogIndexOutOfRange
	}

	mpcAddress, err := b.GetMPCAddress()
	if err != nil {
		return swapInfo, err
	}
	value, memoScript, rightReceiver := b.GetReceivedValue(receipts, mpcAddress, p2pkhType)

	if !rightReceiver {
		return swapInfo, tokens.ErrTxWithWrongReceiver
	}

	bindAddress, toChainId, bindOk := GetBindAddressFromMemoScipt(memoScript)
	if !bindOk {
		log.Debug("wrong memo", "memo", memoScript)
		return swapInfo, tokens.ErrTxWithWrongMemo
	}

	swapInfo.Value = common.BigFromUint64(value)
	swapInfo.Bind = bindAddress
	swapInfo.From = mpcAddress
	swapInfo.ToChainID, err = common.GetBigIntFromStr(toChainId)
	if err != nil {
		return swapInfo, err
	}

	checkErr := b.checkSwapoutInfo(swapInfo)
	if checkErr != nil {
		return swapInfo, checkErr
	}

	if !allowUnstable {
		log.Info("verify swapout pass",
			"token", swapInfo.ERC20SwapInfo.Token, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}

	return swapInfo, nil
}

func (b *Bridge) checkTxStatus(tx *ElectTx, swapInfo *tokens.SwapTxInfo, allowUnstable bool) error {
	txStatus := tx.Status
	if txStatus.BlockHeight != nil {
		swapInfo.Height = *txStatus.BlockHeight // Height
	} else if !*txStatus.Confirmed {
		return tokens.ErrTxNotStable
	} else if *tx.Locktime != 0 {
		// tx with locktime should be on chain, prvent DDOS attack
		return tokens.ErrTxNotStable
	}
	if txStatus.BlockTime != nil {
		swapInfo.Timestamp = *txStatus.BlockTime // Timestamp
	}
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

func (b *Bridge) getSwapTxReceipt(swapInfo *tokens.SwapTxInfo, allowUnstable bool) ([]*ElectTxOut, error) {
	tx, txErr := b.GetTransaction(swapInfo.Hash)
	if txErr != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", swapInfo.Hash, "err", txErr)
		return nil, tokens.ErrTxNotFound
	}
	txres, ok := tx.(*ElectTx)
	if !ok {
		return nil, errTxResultType
	}

	statusErr := b.checkTxStatus(txres, swapInfo, allowUnstable)
	if statusErr != nil {
		return nil, statusErr
	}
	return txres.Vout, nil
}

func (b *Bridge) GetReceivedValue(vout []*ElectTxOut, receiver, pubkeyType string) (value uint64, memoScript string, rightReceiver bool) {
	for _, output := range vout {
		switch *output.ScriptpubkeyType {
		case opReturnType:
			if memoScript == "" {
				memoScript = *output.ScriptpubkeyAsm
			}
			continue
		case pubkeyType:
			if output.ScriptpubkeyAddress == nil || *output.ScriptpubkeyAddress != receiver {
				continue
			}
			rightReceiver = true
			value += *output.Value
		}
	}
	return value, memoScript, rightReceiver
}

// GetBindAddressFromMemoScipt get bind address
func GetBindAddressFromMemoScipt(memoScript string) (bind string, toChainID string, ok bool) {
	parts := regexMemo.Split(memoScript, -1)
	if len(parts) != 2 {
		return "", "", false
	}
	memoHex := strings.TrimSpace(parts[1])
	memo := common.FromHex(memoHex)
	memoStr := string(memo)
	memoArray := strings.Split(memoStr, ":")
	if len(memoArray) != 2 {
		return "", "", false
	}
	bind = memoArray[0]
	toChainID = memoArray[1]
	return bind, toChainID, true
}

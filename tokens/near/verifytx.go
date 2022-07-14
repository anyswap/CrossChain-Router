package near

import (
	"crypto/sha256"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/near/borsh-go"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	txb, ok := rawTx.(*RawTransaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	buf, errb := borsh.Serialize(*txb)
	if errb != nil {
		return errb
	}

	hash := sha256.Sum256(buf)

	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	msgHash := msgHashes[0]
	sigHash := common.ToHex(hash[:])

	if !strings.EqualFold(sigHash, msgHash) {
		logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
		logFunc("message hash mismatch", "want", msgHash, "have", sigHash)
		return tokens.ErrMsgHashMismatch
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

func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	receipts, err := b.getSwapTxReceipt(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	if logIndex >= len(receipts) {
		return swapInfo, tokens.ErrLogIndexOutOfRange
	}

	events, errv := b.fliterReceipts(&receipts[logIndex])
	if errv != nil {
		return swapInfo, tokens.ErrSwapoutLogNotFound
	}

	parseErr := b.parseNep141SwapoutTxEvent(swapInfo, events)
	if parseErr != nil {
		return swapInfo, parseErr
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

func (b *Bridge) checkTxStatus(txres *TransactionResult, allowUnstable bool) error {
	if txres.Status.Failure != nil {
		return tokens.ErrTxIsNotValidated
	}

	if !allowUnstable {
		lastHeight, errh1 := b.GetLatestBlockNumber()
		if errh1 != nil {
			return errh1
		}

		txHeight, errh2 := b.GetBlockNumberByHash(txres.TransactionOutcome.BlockHash)
		if errh2 != nil {
			return errh2
		}

		if lastHeight < txHeight+b.GetChainConfig().Confirmations {
			return tokens.ErrTxNotStable
		}

		if lastHeight < b.ChainConfig.InitialHeight {
			return tokens.ErrTxBeforeInitialHeight
		}
	}
	return nil
}

func (b *Bridge) parseNep141SwapoutTxEvent(swapInfo *tokens.SwapTxInfo, event []string) error {

	swapInfo.ERC20SwapInfo.Token = event[6]
	swapInfo.From = b.GetChainConfig().RouterContract
	swapInfo.Bind = event[8]

	amount, err := common.GetBigIntFromStr(event[1])
	if err != nil {
		return err
	}
	swapInfo.Value = amount

	toChainID, err := common.GetBigIntFromStr(event[9])
	if err != nil {
		return err
	}
	swapInfo.ToChainID = toChainID

	tokenCfg := b.GetTokenConfig(swapInfo.ERC20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	swapInfo.ERC20SwapInfo.TokenID = tokenCfg.TokenID
	swapInfo.To = swapInfo.Bind
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

func (b *Bridge) getSwapTxReceipt(swapInfo *tokens.SwapTxInfo, allowUnstable bool) ([]ReceiptsOutcome, error) {
	tx, txErr := b.GetTransaction(swapInfo.Hash)
	if txErr != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", swapInfo.Hash, "err", txErr)
		return nil, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*TransactionResult)
	if !ok {
		return nil, tokens.ErrTxResultType
	}

	statusErr := b.checkTxStatus(txres, allowUnstable)
	if statusErr != nil {
		return nil, statusErr
	}

	return txres.ReceiptsOutcome, nil
}

func (b *Bridge) fliterReceipts(receipt *ReceiptsOutcome) ([]string, error) {
	if len(receipt.Outcome.Logs) == 2 {
		log_0 := strings.Split(receipt.Outcome.Logs[0], " ")
		log_1 := strings.Split(receipt.Outcome.Logs[1], " ")
		mpcAddress := b.GetChainConfig().RouterContract // in near routerMPC is routerContract
		if len(log_0) != 6 || len(log_1) != 3 || log_0[0] != "Transfer" || log_0[5] != mpcAddress {
			return nil, tokens.ErrSwapoutLogNotFound
		}
		strings.Join(log_0, receipt.Outcome.ExecutorID)
		return append(log_0, log_1...), tokens.ErrSwapoutLogNotFound
	}
	return nil, tokens.ErrSwapoutLogNotFound
}

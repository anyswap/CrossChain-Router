package near

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/near/borsh-go"
)

const (
	SWAPOUTLOG       = "SwapOut"
	SWAPOUTNATIVELOG = "SwapOutNative"
	TRANSFERLOG      = "Transfer"
	TRANSFERV4LOG    = "ft_transfer"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if txb, ok := rawTx.(*RawTransaction); !ok {
		return tokens.ErrWrongRawTx
	} else {
		if buf, err := borsh.Serialize(*txb); err != nil {
			return err
		} else {
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
	}
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

	events, err := b.fliterReceipts(swapInfo, &receipts[logIndex])
	if err != nil {
		return swapInfo, err
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
			"token", swapInfo.ERC20SwapInfo.Token, "from", swapInfo.From,
			"txto", swapInfo.TxTo, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}

	return swapInfo, nil
}

func (b *Bridge) checkTxStatus(swapInfo *tokens.SwapTxInfo, txres *TransactionResult, allowUnstable bool) error {
	if txres.Status.Failure != nil || txres.Status.SuccessValue == nil {
		log.Warn("Near tx status is not success", "result", txres.Status.Failure)
		return tokens.ErrTxWithWrongStatus
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
		swapInfo.Height = txHeight

		if lastHeight < txHeight+b.GetChainConfig().Confirmations {
			return tokens.ErrTxNotStable
		}

		if txHeight < b.ChainConfig.InitialHeight {
			return tokens.ErrTxBeforeInitialHeight
		}
	}
	return nil
}

func (b *Bridge) parseNep141SwapoutTxEvent(swapInfo *tokens.SwapTxInfo, event []string) (err error) {
	if len(event) != 5 {
		return tokens.ErrSwapoutLogNotFound
	}
	if err := b.parseTxEvent(swapInfo, event); err != nil {
		return err
	} else {
		tokenCfg := b.GetTokenConfig(swapInfo.ERC20SwapInfo.Token)
		if tokenCfg == nil {
			return tokens.ErrMissTokenConfig
		}
		swapInfo.ERC20SwapInfo.TokenID = tokenCfg.TokenID
		swapInfo.To = swapInfo.Bind
		return nil
	}
}

func (b *Bridge) parseTxEvent(swapInfo *tokens.SwapTxInfo, event []string) error {
	swapInfo.ERC20SwapInfo.Token = event[1]
	swapInfo.Bind = event[2]

	amount, err := common.GetBigIntFromStr(event[3])
	if err != nil {
		return err
	}
	swapInfo.Value = amount

	toChainID, err := common.GetBigIntFromStr(event[4])
	if err != nil {
		return err
	}
	swapInfo.ToChainID = toChainID
	return nil
}

func (b *Bridge) checkSwapoutInfo(swapInfo *tokens.SwapTxInfo) error {
	if strings.EqualFold(swapInfo.From, swapInfo.To) {
		return tokens.ErrTxWithWrongSender
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

	statusErr := b.checkTxStatus(swapInfo, txres, allowUnstable)
	if statusErr != nil {
		return nil, statusErr
	}

	swapInfo.From = txres.Transaction.SignerID
	swapInfo.TxTo = txres.Transaction.ReceiverID

	return txres.ReceiptsOutcome, nil
}

func (b *Bridge) fliterReceipts(swapInfo *tokens.SwapTxInfo, receipt *ReceiptsOutcome) ([]string, error) {
	mpcAddress := b.GetRouterContract("")
	executorID := receipt.Outcome.ExecutorID
	if tokenCfg := b.GetTokenConfig(executorID); tokenCfg == nil {
		return nil, tokens.ErrMissTokenConfig
	} else {
		switch tokenCfg.ContractVersion {
		case 666:
			if len(receipt.Outcome.Logs) == 1 {
				log := strings.Split(receipt.Outcome.Logs[0], " ")
				if log[0] == SWAPOUTLOG && len(log) == 9 {
					return []string{SWAPOUTLOG, executorID, log[4], log[6], log[8]}, nil
				}
			}
		case 999:
			if len(receipt.Outcome.Logs) == 1 {
				log := strings.Split(receipt.Outcome.Logs[0], " ")
				if log[0] == SWAPOUTNATIVELOG && len(log) == 9 && executorID == mpcAddress {
					return []string{SWAPOUTNATIVELOG, executorID, log[4], log[6], log[8]}, nil
				}
			}
		default:
			switch len(receipt.Outcome.Logs) {
			case 1:
				var event Nep141V4TransferEvent
				if err := json.Unmarshal([]byte(receipt.Outcome.Logs[0][11:]), &event); err == nil {
					if len(event.Data) == 1 {
						log := strings.Split(event.Data[0].Memo, " ")
						if event.Event == TRANSFERV4LOG && len(log) == 2 && event.Data[0].NewOwnerId == mpcAddress {
							swapInfo.From = event.Data[0].OldOwnerId
							return []string{TRANSFERLOG, executorID, log[0], event.Data[0].Amount, log[1]}, nil
						}
					}
				}
			case 2:
				log_0 := strings.Split(receipt.Outcome.Logs[0], " ")
				log_1 := strings.Split(receipt.Outcome.Logs[1], " ")
				if len(log_0) == 6 && len(log_1) == 3 && log_0[0] == TRANSFERLOG && log_0[5] == mpcAddress {
					swapInfo.From = log_0[3]
					return []string{TRANSFERLOG, executorID, log_1[1], log_0[1], log_1[2]}, nil
				}
			case 3:
				log_0 := strings.Split(receipt.Outcome.Logs[0], " ")
				log_1 := strings.Split(receipt.Outcome.Logs[1], " ")
				if len(log_0) == 6 && len(log_1) == 3 && log_0[0] == TRANSFERLOG && log_0[5] == mpcAddress {
					callSuccessPattern := fmt.Sprintf("Transfer amount %v to %v success with memo:", log_0[1], mpcAddress)
					if !strings.HasPrefix(receipt.Outcome.Logs[2], callSuccessPattern) {
						return nil, tokens.ErrSwapoutPatternMismatch
					}
					swapInfo.From = log_0[3]
					return []string{TRANSFERLOG, executorID, log_1[1], log_0[1], log_1[2]}, nil
				}
			default:
				return nil, tokens.ErrSwapoutLogNotFound
			}
		}
	}
	return nil, tokens.ErrSwapoutLogNotFound
}

package cosmosRouter

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
	"github.com/cosmos/cosmos-sdk/types"
)

const (
	TransferType    = "transfer"
	IbcTransferType = "ibc_transfer"
	SenderKey       = "sender"
	RecipientKey    = "recipient"
	AmountKey       = "amount"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	if multichainTx, ok := rawTx.(*cosmosSDK.BuildRawTx); !ok {
		return tokens.ErrWrongRawTx
	} else {
		txBuilder := multichainTx.TxBuilder
		extra := multichainTx.Extra
		mpc := b.GetRouterContract("")
		mpcPubkey := router.GetMPCPublicKey(mpc)
		if mpcPubkey == "" {
			return tokens.ErrMissMPCPublicKey
		}

		if signBytes, err := b.CosmosRestClient.GetSignBytes(*txBuilder, *extra.AccountNum, *extra.Sequence); err != nil {
			return err
		} else {
			msgHash := fmt.Sprintf("%X", cosmosSDK.Sha256Sum(signBytes))
			if !strings.EqualFold(msgHash, msgHashes[0]) {
				log.Warn("message hash mismatch",
					"want", msgHashes[0], "have", string(signBytes))
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

	if txr, err := b.GetTransactionByHash(txHash); err != nil {
		log.Debug("[verifySwapin] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return swapInfo, tokens.ErrTxNotFound
	} else {
		if txHeight, err := b.checkTxStatus(txr, allowUnstable); err != nil {
			return swapInfo, err
		} else {
			swapInfo.Height = txHeight
		}

		if err := ParseMemo(swapInfo, txr.Tx.Body.Memo); err != nil {
			return swapInfo, err
		}

		if err := b.ParseAmountTotal(txr.TxResponse.Logs[logIndex-1], swapInfo); err != nil {
			return swapInfo, err
		}

		if checkErr := b.checkSwapoutInfo(swapInfo); checkErr != nil {
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

func (b *Bridge) checkTxStatus(txres *cosmosSDK.GetTxResponse, allowUnstable bool) (txHeight uint64, err error) {
	if txHeight, err := strconv.ParseUint(txres.TxResponse.Height, 10, 64); err != nil {
		return 0, nil
	} else {
		if txres.TxResponse.Code != 0 {
			return txHeight, tokens.ErrTxWithWrongStatus
		}

		if !allowUnstable {
			if h, err := b.GetLatestBlockNumber(); err != nil {
				return txHeight, err
			} else {
				if h < txHeight+b.GetChainConfig().Confirmations {
					return txHeight, tokens.ErrTxNotStable
				}
				if h < b.ChainConfig.InitialHeight {
					return txHeight, tokens.ErrTxBeforeInitialHeight
				}
			}
		}
		return txHeight, err
	}
}

func ParseMemo(swapInfo *tokens.SwapTxInfo, memo string) error {
	fields := strings.Split(memo, ":")
	if len(fields) == 2 {
		if toChainID, err := common.GetBigIntFromStr(fields[1]); err != nil {
			return err
		} else {
			dstBridge := router.GetBridgeByChainID(toChainID.String())
			if dstBridge != nil && dstBridge.IsValidAddress(fields[0]) {
				swapInfo.Bind = fields[0]      // Bind
				swapInfo.ToChainID = toChainID // ToChainID
				swapInfo.To = swapInfo.Bind    // To
				return nil
			}
		}
	}
	return tokens.ErrTxWithWrongMemo
}

//nolint:goconst // allow big check logic
func (b *Bridge) ParseAmountTotal(messageLog types.ABCIMessageLog, swapInfo *tokens.SwapTxInfo) error {
	value := big.NewInt(0)
	unit := ""
	events := messageLog.Events
	if len(events) == 4 && events[3].Type == TransferType && len(events[3].Attributes) == 3 {
		transferEventDetails := events[3].Attributes
		b.ParseCoinAmount(value, swapInfo, transferEventDetails[1], transferEventDetails[0], transferEventDetails[2], &unit)
	} else if len(messageLog.Events) == 6 && events[2].Type == IbcTransferType && len(events[2].Attributes) == 2 && len(events[5].Attributes) == 3 {
		//to do
		//check ibc receipt
		ibcEventDetails := events[2].Attributes
		transferEventDetails := events[5].Attributes
		b.ParseCoinAmount(value, swapInfo, ibcEventDetails[0], ibcEventDetails[1], transferEventDetails[2], &unit)
	}
	if value.Cmp(big.NewInt(0)) > 0 {
		swapInfo.Value = value
		swapInfo.ERC20SwapInfo.Token = unit
		if tokenCfg := b.GetTokenConfig(swapInfo.ERC20SwapInfo.Token); tokenCfg == nil {
			return tokens.ErrMissTokenConfig
		} else {
			swapInfo.ERC20SwapInfo.TokenID = tokenCfg.TokenID
			return nil
		}
	}
	return tokens.ErrTxWithWrongValue
}

func (b *Bridge) ParseCoinAmount(value *big.Int, swapInfo *tokens.SwapTxInfo, sender, recipient, amount types.Attribute, unit *string) {
	mpc := b.GetRouterContract("")
	if sender.Key == SenderKey &&
		recipient.Key == RecipientKey &&
		amount.Key == AmountKey {
		// receiver mismatch
		if common.IsEqualIgnoreCase(recipient.Value, mpc) {
			if recvCoins, err := cosmosSDK.ParseCoinsNormalized(amount.Value); err == nil {
				for _, coin := range recvCoins {
					if *unit == "" || *unit == coin.Denom {
						if swapInfo.From == "" {
							swapInfo.From = sender.Value
						}
						*unit = coin.Denom
						recvAmount := recvCoins.AmountOfNoDenomValidation(*unit)
						if !recvAmount.IsNil() && !recvAmount.IsZero() {
							value.Add(value, recvAmount.BigInt())
						}
					}
				}
			}
		}
	}
}

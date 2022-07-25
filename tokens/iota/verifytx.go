package iota

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	SWAPOUT = "swapOut"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	return
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
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = 0                             // LogIndex always 0 (do not support multiple in one tx)
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	if tx, err := b.GetTransactionMetadata(txHash); err != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransactionMetadata fail", "tx", txHash, "err", err)
		return swapInfo, tokens.ErrTxNotFound
	} else {
		if !allowUnstable {
			if h, err := b.GetLatestBlockNumber(); err != nil {
				return swapInfo, err
			} else {
				if h < uint64(*tx.MilestoneIndex)+b.GetChainConfig().Confirmations {
					return swapInfo, tokens.ErrTxNotStable
				}
				if h < b.ChainConfig.InitialHeight {
					return swapInfo, tokens.ErrTxBeforeInitialHeight
				}
			}
		}
	}

	if txRes, err := b.GetTransaction(txHash); err != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return swapInfo, tokens.ErrTxNotFound
	} else {
		if txres, ok := txRes.(*iotago.Message); !ok {
			return swapInfo, tokens.ErrTxResultType
		} else {
			if payloadRaw, err := txres.Payload.MarshalJSON(); err != nil {
				return swapInfo, err
			} else {
				if err := b.ParseMessagePayload(swapInfo, payloadRaw); err != nil {
					return swapInfo, err
				} else {
					if err := b.checkSwapoutInfo(swapInfo); err != nil {
						return swapInfo, err
					}
				}
			}
		}
	}

	if !allowUnstable {
		log.Info("verify swapout pass",
			"token", swapInfo.ERC20SwapInfo.Token, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}

	return nil, nil
}

func (b *Bridge) ParseMessagePayload(swapInfo *tokens.SwapTxInfo, payload []byte) error {
	var messagePayload MessagePayload
	if err := json.Unmarshal(payload, &messagePayload); err != nil {
		return err
	} else {
		mpc := b.GetRouterContract("")
		var amount uint64
		if messagePayload.Type != 0 {
			return tokens.ErrPayloadType
		}
		for _, output := range messagePayload.Essence.Outputs {
			if output.Address.Address == mpc {
				amount += output.Amount
			}
		}
		if amount == 0 {
			return tokens.ErrTxWithWrongValue
		} else {
			swapInfo.Value = common.BigFromUint64(amount)
			swapInfo.From = mpc
			if bind, toChainId, err := ParseIndexPayload(messagePayload.Essence.Payload); err != nil {
				return err
			} else {
				swapInfo.Bind = bind
				swapInfo.To = swapInfo.Bind
				if toChainID, err := common.GetBigIntFromStr(toChainId); err != nil {
					return err
				} else {
					swapInfo.ToChainID = toChainID
				}
			}
		}
	}
	return nil
}

func ParseIndexPayload(payload Payload) (string, string, error) {
	if payload.Type != 2 {
		return "", "", tokens.ErrPayloadType
	}
	if index, err := hex.DecodeString(payload.Index); err != nil || string(index) != SWAPOUT {
		return "", "", tokens.ErrPayloadType
	}
	if data, err := hex.DecodeString(payload.Data); err != nil {
		return "", "", tokens.ErrPayloadType
	} else {
		if fields := strings.Split(string(data), ":"); len(fields) != 2 {
			return "", "", tokens.ErrPayloadType
		} else {
			return fields[0], fields[1], nil
		}
	}
}

func (b *Bridge) checkSwapoutInfo(swapInfo *tokens.SwapTxInfo) error {
	return nil
}

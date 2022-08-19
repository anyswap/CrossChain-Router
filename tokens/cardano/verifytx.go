package cardano

import (
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
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
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = 0                             // LogIndex always 0 (do not support multiple in one tx)
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	if outputs, err := b.getTxOutputs(swapInfo, allowUnstable); err != nil {
		return swapInfo, err
	} else {
		if outputsList := b.fliterTxOutputs(outputs); len(outputsList) == 0 {
			return swapInfo, tokens.ErrOutputLength
		} else {
			if err := b.parseTxOutputs(swapInfo, outputsList); err != nil {
				return nil, err
			}
		}
	}

	if !allowUnstable {
		log.Info("verify swapout pass",
			"token", swapInfo.ERC20SwapInfo.Token, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}

	return swapInfo, tokens.ErrNotImplemented
}

func (b *Bridge) getTxOutputs(swapInfo *tokens.SwapTxInfo, allowUnstable bool) ([]Output, error) {
	if tx, txErr := b.GetTransaction(swapInfo.Hash); txErr != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", swapInfo.Hash, "err", txErr)
		return nil, tokens.ErrTxNotFound
	} else {
		if txres, ok := tx.(*Transaction); !ok {
			return nil, tokens.ErrTxResultType
		} else {
			statusErr := b.checkTxStatus(txres, allowUnstable)
			if statusErr != nil {
				return nil, statusErr
			}
			return txres.Outputs, nil
		}
	}
}

func (b *Bridge) parseTxOutputs(swapInfo *tokens.SwapTxInfo, outputs []Output) error {
	assetMap := make(map[string]uint64)
	for _, output := range outputs {
		for _, token := range output.Tokens {
			if number, err := strconv.ParseUint(token.Quantity, 10, 64); err != nil {
				return err
			} else {
				assetMap[token.Asset.AssetId] = assetMap[token.Asset.AssetId] + number
			}
		}
	}
	log.Info("todo", "assetMap", assetMap)
	return nil
}

func (b *Bridge) checkTxStatus(txres *Transaction, allowUnstable bool) error {
	if !txres.ValidContract {
		return tokens.ErrTxIsNotValidated
	}

	if !allowUnstable {
		lastHeight, errh1 := b.GetLatestBlockNumber()
		if errh1 != nil {
			return errh1
		}

		if lastHeight < txres.Block.Number+b.GetChainConfig().Confirmations {
			return tokens.ErrTxNotStable
		}

		if lastHeight < b.ChainConfig.InitialHeight {
			return tokens.ErrTxBeforeInitialHeight
		}
	}
	return nil
}

func (b *Bridge) fliterTxOutputs(outputs []Output) []Output {
	mpc := b.GetRouterContract("")
	var outputsList []Output
	for _, output := range outputs {
		if output.Address == mpc {
			outputsList = append(outputsList, output)
		}
	}
	return outputsList
}

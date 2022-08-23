package cardano

import (
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
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
func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex always 0 (do not support multiple in one tx)
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	if outputs, metadata, err := b.getTxOutputs(swapInfo, allowUnstable); err != nil {
		return swapInfo, err
	} else {
		tempIndex := 0
		outputIndex := 0
		assetIndex := 0
		for index, output := range outputs {
			if logIndex > tempIndex+len(output.Tokens) {
				tempIndex += len(output.Tokens)
			} else {
				outputIndex = index
				assetIndex = logIndex - tempIndex
			}
		}
		if tokenInfo, err := b.parseTxOutput(outputs[outputIndex], assetIndex); err != nil {
			return swapInfo, err
		} else {
			if err := b.parseTokenInfo(swapInfo, tokenInfo, metadata); err != nil {
				return nil, err
			} else {
				if err := b.parseTokenInfo(swapInfo, tokenInfo, metadata); err != nil {
					return nil, err
				}
				if err := b.checkSwapoutInfo(swapInfo); err != nil {
					return swapInfo, err
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
	}
}

func (b *Bridge) getTxOutputs(swapInfo *tokens.SwapTxInfo, allowUnstable bool) ([]Output, *Metadata, error) {
	if tx, txErr := b.GetTransaction(swapInfo.Hash); txErr != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", swapInfo.Hash, "err", txErr)
		return nil, nil, tokens.ErrTxNotFound
	} else {
		if txres, ok := tx.(*Transaction); !ok {
			return nil, nil, tokens.ErrTxResultType
		} else {
			statusErr := b.checkTxStatus(txres, allowUnstable)
			if statusErr != nil {
				return nil, nil, statusErr
			}
			for index, metadata := range txres.Metadata {
				if metadata.Key == "123" {
					return txres.Outputs, &txres.Metadata[index], nil
				}
			}
			return nil, nil, tokens.ErrMetadataKeyMissMatch
		}
	}
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

func (b *Bridge) parseTxOutput(output Output, logIndex int) (*Token, error) {
	mpc := b.GetRouterContract("")
	if output.Address == mpc {
		return &output.Tokens[logIndex-1], nil
	} else {
		return nil, tokens.ErrMpcAddrMissMatch
	}
}

func (b *Bridge) parseTokenInfo(swapInfo *tokens.SwapTxInfo, tokenInfo *Token, metadata *Metadata) error {
	mpc := b.GetRouterContract("")
	swapInfo.ERC20SwapInfo.Token = strings.Replace(tokenInfo.Asset.AssetId, tokenInfo.Asset.AssetName, "."+tokenInfo.Asset.AssetName, 1)
	swapInfo.From = mpc
	swapInfo.Bind = metadata.Value.Bind

	amount, erra := common.GetBigIntFromStr(tokenInfo.Quantity)
	if erra != nil {
		return erra
	}
	swapInfo.Value = amount

	swapInfo.ToChainID = big.NewInt(int64(metadata.Value.ToChainId))

	tokenCfg := b.GetTokenConfig(swapInfo.ERC20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	swapInfo.ERC20SwapInfo.TokenID = tokenCfg.TokenID

	swapInfo.To = metadata.Value.Bind
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

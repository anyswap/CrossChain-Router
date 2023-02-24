package cardano

import (
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	MetadataKey = "123"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if rawTransaction, ok := rawTx.(*RawTransaction); !ok {
		return tokens.ErrWrongRawTx
	} else {
		tx, err := b.CreateRawTx(rawTransaction, b.GetRouterContract(""))
		if err != nil {
			return tokens.ErrWrongRawTx
		}
		txhash, err := tx.Hash()
		if err != nil {
			return tokens.ErrWrongRawTx
		}
		if txhash.String() != msgHashes[0] {
			return tokens.ErrMsgHashMismatch
		}
		return nil
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

//nolint:gocyclo,funlen // ok
func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	if outputs, metadata, err := b.getTxOutputs(swapInfo, allowUnstable); err != nil {
		return swapInfo, err
	} else {
		tempIndex := 0
		outputIndex := 0
		assetIndex := 0
		for index, output := range outputs {
			if index != int(output.Index) {
				return nil, tokens.ErrOutputIndexSort
			}
			if logIndex > tempIndex+len(output.Tokens)+1 {
				tempIndex += len(output.Tokens) + 1
			} else {
				outputIndex = index
				assetIndex = logIndex - tempIndex - 1
				break
			}
		}
		if tokenInfo, err := b.parseTxOutput(outputs[outputIndex], assetIndex); err != nil {
			return swapInfo, err
		} else {
			if err := b.parseTokenInfo(swapInfo, tokenInfo, metadata); err != nil {
				return nil, err
			} else {
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
				if metadata.Key == MetadataKey {
					if len(txres.Inputs) > 0 {
						swapInfo.From = txres.Inputs[0].Address
					}
					return txres.Outputs, &txres.Metadata[index], nil
				}
			}
			return nil, nil, tokens.ErrMetadataKeyMissMatch
		}
	}
}

func (b *Bridge) checkTxStatus(txres *Transaction, allowUnstable bool) error {
	// TODO what's ValidContract mean?
	if !txres.ValidContract {
		return tokens.ErrTxIsNotValidated
	}
	// allowUnstable is true only in scan and register swap tx
	//	no need to consider block rollback in register
	//	because the tx need to be reverified in verify job
	// when start verify job, allowUnstable is false,
	//	and we must check the block confirmations is enough
	if allowUnstable {
		return nil
	}
	if lastHeight, err := b.GetLatestBlockNumber(); err != nil {
		return err
	} else {
		//According to the IOHK, rollbacks of 20 blocks or higher are categorized as very deep and are the most unlikely to occur.
		txHeight := txres.Block.Number
		if lastHeight < txHeight+b.GetChainConfig().Confirmations {
			return tokens.ErrTxNotStable
		}
		if txHeight < b.ChainConfig.InitialHeight {
			return tokens.ErrTxBeforeInitialHeight
		}
	}
	return nil
}

func (b *Bridge) parseTxOutput(output Output, logIndex int) (*Token, error) {
	mpc := b.GetRouterContract("")
	if output.Address == mpc {
		if logIndex == 0 {
			if amount, err := common.GetBigIntFromStr(output.Value); err != nil || amount.Cmp(DefaultAdaAmount) <= 0 {
				return nil, tokens.ErrAdaSwapOutAmount
			} else {
				return &Token{
					Asset: Asset{
						PolicyId:  AdaAsset,
						AssetName: AdaAsset,
					},
					Quantity: amount.Sub(amount, DefaultAdaAmount).String(),
				}, nil
			}
		}
		return &output.Tokens[logIndex-1], nil
	} else {
		return nil, tokens.ErrMpcAddrMissMatch
	}
}

func (b *Bridge) parseTokenInfo(swapInfo *tokens.SwapTxInfo, tokenInfo *Token, metadata *Metadata) error {
	amount, err := common.GetBigIntFromStr(tokenInfo.Quantity)
	if err != nil {
		return err
	}

	swapInfo.Value = amount

	if tokenInfo.Asset.PolicyId == AdaAsset {
		swapInfo.ERC20SwapInfo.Token = AdaAsset
	} else {
		swapInfo.ERC20SwapInfo.Token = tokenInfo.Asset.PolicyId + "." + tokenInfo.Asset.AssetName
	}
	swapInfo.Bind = metadata.Value.Bind

	if tochainId, err := common.GetBigIntFromStr(metadata.Value.ToChainId); err != nil {
		return err
	} else {
		swapInfo.ToChainID = tochainId
	}

	tokenCfg := b.GetTokenConfig(swapInfo.ERC20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	swapInfo.ERC20SwapInfo.TokenID = tokenCfg.TokenID

	swapInfo.To = metadata.Value.Bind
	return nil
}

func (b *Bridge) checkSwapoutInfo(swapInfo *tokens.SwapTxInfo) error {
	if swapInfo.From != "" && strings.EqualFold(swapInfo.From, swapInfo.To) {
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

	bindAddr := swapInfo.Bind
	if !toBridge.IsValidAddress(bindAddr) {
		log.Warn("wrong bind address in swapin", "bind", bindAddr)
		return tokens.ErrWrongBindAddress
	}

	if !tokens.CheckTokenSwapValue(swapInfo, fromTokenCfg.Decimals, toTokenCfg.Decimals) {
		return tokens.ErrTxWithWrongValue
	}
	return nil
}

package cardano

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// BuildRawTransaction build raw tx
//
//nolint:funlen,gocyclo // ok
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if extra, err := b.initExtra(args); err != nil {
		return nil, err
	} else {
		if extra.RawTx != nil {
			var tx RawTransaction
			if err := json.Unmarshal(extra.RawTx, &tx); err != nil {
				return nil, err
			}
			if err := b.VerifyRawTransaction(&tx, args); err != nil {
				return nil, err
			}
			return &tx, nil
		}
		if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
			return nil, tokens.ErrToChainIDMismatch
		}
		if args.Input != nil {
			return nil, fmt.Errorf("forbid build raw swap tx with input data")
		}
		if args.From == "" {
			return nil, fmt.Errorf("forbid empty sender")
		}

		routerMPC := b.GetRouterContract("")
		if !common.IsEqualIgnoreCase(args.From, routerMPC) {
			log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
			return nil, tokens.ErrSenderMismatch
		}

		mpcPubkey := router.GetMPCPublicKey(args.From)
		if mpcPubkey == "" {
			return nil, tokens.ErrMissMPCPublicKey
		}
		erc20SwapInfo := args.ERC20SwapInfo
		multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, args.ToChainID.String())
		if multichainToken == "" {
			log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
			return nil, tokens.ErrMissTokenConfig
		}

		tokenCfg := b.GetTokenConfig(multichainToken)
		if tokenCfg == nil {
			return nil, tokens.ErrMissTokenConfig
		}

		if receiver, amount, err := b.getReceiverAndAmount(args, multichainToken); err != nil {
			return nil, err
		} else {
			args.SwapValue = amount // SwapValue
			if utxos, err := b.QueryUtxo(routerMPC, multichainToken, amount); err != nil {
				return nil, err
			} else {
				swapId := fmt.Sprintf("%s-%d", args.SwapID, args.LogIndex)
				if rawTransaction, err := b.BuildTx(swapId, receiver, multichainToken, amount, utxos); err != nil {
					return nil, err
				} else {
					if err := CreateRawTx(rawTransaction, routerMPC); err != nil {
						return nil, err
					} else {
						if minFee, err := CalcMinFee(rawTransaction); err != nil {
							return nil, err
						} else {
							if feeList := strings.Split(minFee, " "); len(feeList) != 2 {
								return nil, errors.New("feeList length not match")
							} else {
								rawTransaction.Fee = feeList[0]
								if adaAmount, err := common.GetBigIntFromStr(rawTransaction.TxOuts[args.From][AdaAsset]); err != nil {
									return nil, err
								} else {
									if feeAmount, err := common.GetBigIntFromStr(feeList[0]); err != nil {
										return nil, err
									} else {
										returnAmount := adaAmount.Sub(adaAmount, feeAmount)
										if returnAmount.Cmp(FixAdaAmount) < 0 {
											return nil, errors.New("return value less than min value")
										} else {
											rawTransaction.TxOuts[args.From][AdaAsset] = returnAmount.String()
											if err := CreateRawTx(rawTransaction, routerMPC); err != nil {
												return nil, err
											} else {
												if rawBytes, err := json.Marshal(rawTransaction); err != nil {
													return nil, err
												} else {
													extra.RawTx = rawBytes
													return rawTransaction, nil
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func (b *Bridge) BuildTx(swapId, receiver, assetId string, amount *big.Int, utxos map[UtxoKey]AssetsMap) (*RawTransaction, error) {
	log.Infof("build Tx:\nreceiver:%+v\nassetId:%+v\namount:%+v\nutxos:%+v\n", receiver, assetId, amount, utxos)
	routerMpc := b.GetRouterContract("")
	rawTransaction := &RawTransaction{
		Fee:     "0",
		OutFile: swapId,
		TxOuts:  make(map[string]AssetsMap),
		TxIns:   []UtxoKey{},
	}
	allAssetsMap := map[string]uint64{}
	for utxoKey, assetsMap := range utxos {
		rawTransaction.TxIns = append(rawTransaction.TxIns, utxoKey)
		for asset, assetAmount := range assetsMap {
			if value, err := common.GetBigIntFromStr(assetAmount); err != nil {
				return nil, err
			} else {
				allAssetsMap[asset] += value.Uint64()
			}
		}
	}

	rawTransaction.TxOuts[receiver] = map[string]string{}
	rawTransaction.TxOuts[routerMpc] = map[string]string{}
	var adaAmount *big.Int
	if assetId == AdaAsset {
		adaAmount = amount
	} else {
		adaAmount = FixAdaAmount
		policyId := strings.Split(assetId, ".")[0]
		if allAssetsMap[assetId] >= amount.Uint64() {
			rawTransaction.TxOuts[receiver][assetId] = amount.String()
			if allAssetsMap[assetId] > amount.Uint64() {
				rawTransaction.TxOuts[routerMpc][assetId] = fmt.Sprint((allAssetsMap[assetId] - amount.Uint64()))
			}
		} else {
			if policyId != MPCPolicyId {
				return nil, tokens.ErrTokenBalancesNotEnough
			} else {
				rawTransaction.Mint = map[string]string{
					assetId: fmt.Sprint(amount.Uint64() - allAssetsMap[assetId]),
				}
				rawTransaction.TxOuts[receiver][assetId] = amount.String()
			}
		}
	}
	if adaAmount.Cmp(FixAdaAmount) < 0 {
		return nil, tokens.ErrAdaSwapOutAmount
	}
	rawTransaction.TxOuts[receiver][AdaAsset] = adaAmount.String()
	if allAssetsMap[AdaAsset] > adaAmount.Uint64() {
		rawTransaction.TxOuts[routerMpc][AdaAsset] = fmt.Sprint((allAssetsMap[AdaAsset] - adaAmount.Uint64()))
	}
	for assetIdWithName, assetAmount := range allAssetsMap {
		if assetIdWithName != AdaAsset && assetIdWithName != assetId {
			rawTransaction.TxOuts[routerMpc][assetIdWithName] = fmt.Sprint(assetAmount)
		}
	}
	return rawTransaction, nil
}

func CreateRawTx(rawTransaction *RawTransaction, mpcAddr string) error {
	cmdString := ""
	inputString := ""
	for _, utxoKey := range rawTransaction.TxIns {
		inputString = fmt.Sprintf("%s  --tx-in  %s#%d", inputString, strings.TrimSpace(utxoKey.TxHash), utxoKey.TxIndex)
	}
	outputString := ""
	if len(rawTransaction.TxOuts) > 2 {
		return tokens.ErrOutputLength
	}
	tempIndex := uint64(0)
	for address, assets := range rawTransaction.TxOuts {
		if address == mpcAddr {
			rawTransaction.TxIndex = tempIndex
		} else {
			tempIndex++
		}
		outputString = fmt.Sprintf("%s  --tx-out  %s+%s", outputString, strings.TrimSpace(address), strings.TrimSpace(assets[AdaAsset]))
		for asset, amount := range assets {
			if asset != AdaAsset {
				outputString = fmt.Sprintf("%s+%s %s", outputString, strings.TrimSpace(amount), strings.TrimSpace(asset))
			}
		}
	}
	if rawTransaction.Mint != nil {
		mintString := ""
		for asset, amount := range rawTransaction.Mint {
			mintString = fmt.Sprintf("%s  --mint=%s %s", mintString, strings.TrimSpace(amount), strings.TrimSpace(asset))
		}
		cmdString = fmt.Sprintf(BuildRawTxWithMintCmd, rawTransaction.Fee, inputString, outputString, mintString, RawPath+rawTransaction.OutFile+RawSuffix)
	} else {
		cmdString = fmt.Sprintf(BuildRawTxWithoutMintCmd, rawTransaction.Fee, inputString, outputString, RawPath+rawTransaction.OutFile+RawSuffix)
	}

	log.Info("CardanoExecCmd", "cmdString", cmdString)
	if _, err := ExecCmd(cmdString, "  "); err != nil {
		return err
	}
	return nil
}

func CalcMinFee(rawTransaction *RawTransaction) (string, error) {
	txBodyPath := RawPath + rawTransaction.OutFile + RawSuffix
	cmdString := fmt.Sprintf(CalcMinFeeCmd, txBodyPath, len(rawTransaction.TxIns), len(rawTransaction.TxOuts))
	if execRes, err := ExecCmd(cmdString, " "); err != nil {
		return "", err
	} else {
		return execRes, nil
	}
}

func (b *Bridge) initExtra(args *tokens.BuildTxArgs) (extra *tokens.AllExtras, err error) {
	extra = args.Extra
	if extra == nil {
		extra = &tokens.AllExtras{}
		args.Extra = extra
	}
	if extra.Sequence == nil {
		extra.Sequence, err = b.GetSeq(args)
		if err != nil {
			return nil, err
		}
	}
	return extra, nil
}

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	return 0, nil
}

// GetSeq returns account tx sequence
func (b *Bridge) GetSeq(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

	if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) { // increase automatically
		nonce = b.GetSwapNonce(args.From)
		return &nonce, nil
	}

	nonce, err = b.GetPoolNonce(args.From, "pending")
	if err != nil {
		return nil, err
	}
	nonce = b.AdjustNonce(args.From, nonce)
	return &nonce, nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount *big.Int, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
	if !b.IsValidAddress(receiver) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("swapout to invalid receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	if fromBridge == nil {
		return receiver, amount, tokens.ErrNoBridgeForChainID
	}
	fromTokenCfg := fromBridge.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", erc20SwapInfo.Token)
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	amount = tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	return receiver, amount, err
}

func (b *Bridge) QueryUtxo(address, assetName string, amount *big.Int) (map[UtxoKey]AssetsMap, error) {
	if utxos, err := b.GetTransactionChainingMap(assetName, amount); err != nil {
		return b.QueryUtxoOnChain(address)
	} else {
		return utxos, nil
	}
}

func (b *Bridge) GetTransactionChainingMap(assetName string, amount *big.Int) (map[UtxoKey]AssetsMap, error) {
	utxos := make(map[UtxoKey]AssetsMap)
	needAmount := big.NewInt(amount.Int64())
	assetBalance := TransactionChaining.AssetsMap[assetName]
	if assetBalance != "" {
		if balance, err := common.GetBigIntFromStr(assetBalance); err != nil {
			return nil, err
		} else {
			if assetName == AdaAsset {
				needAmount.Add(needAmount, DefaultAdaAmount)
			}
			if balance.Cmp(needAmount) >= 0 {
				utxoKey := UtxoKey{TxHash: TransactionChaining.InputKey.TxHash, TxIndex: TransactionChaining.InputKey.TxIndex}
				utxos[utxoKey] = TransactionChaining.AssetsMap
				return utxos, nil
			}
		}
	}
	return nil, tokens.ErrTokenBalancesNotEnough
}

func (b *Bridge) QueryUtxoOnChain(address string) (map[UtxoKey]AssetsMap, error) {
	utxos := make(map[UtxoKey]AssetsMap)
	if outputs, err := b.GetUtxosByAddress(address); err != nil {
		return nil, err
	} else {
		for _, output := range *outputs {
			utxoKey := UtxoKey{TxHash: output.TxHash, TxIndex: output.Index}
			if !TransactionChainingKeyCache.SpentUtxoMap[utxoKey] {
				utxos[utxoKey] = make(AssetsMap)
				utxos[utxoKey][AdaAsset] = output.Value
				for _, token := range output.Tokens {
					utxos[utxoKey][token.Asset.PolicyId+"."+token.Asset.AssetName] = token.Quantity
				}
			}
		}
		return utxos, nil
	}
}

func (b *Bridge) VerifyRawTransaction(raw *RawTransaction, args *tokens.BuildTxArgs) error {
	mpcAddr := b.GetRouterContract("")
	if len(raw.TxOuts) > 2 {
		return tokens.ErrOutputLength
	}

	mpcAssetsMap := raw.TxOuts[mpcAddr]
	receiverAssetsMap := raw.TxOuts[args.SwapArgs.Bind]
	if mpcAssetsMap == nil || receiverAssetsMap == nil {
		return tokens.ErrTxWithWrongReceiver
	}
	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return tokens.ErrMissTokenConfig
	}

	switch len(receiverAssetsMap) {
	case 1:
		adaAmount := receiverAssetsMap[AdaAsset]
		if value, err := common.GetBigIntFromStr(adaAmount); err != nil {
			return err
		} else {
			if value.Cmp(args.OriginValue) > 0 {
				return tokens.ErrTxWithWrongValue
			}
		}
	case 2:
		adaAmount := receiverAssetsMap[AdaAsset]
		if value, err := common.GetBigIntFromStr(adaAmount); err != nil {
			return err
		} else {
			if value.Cmp(DefaultAdaAmount) > 0 {
				return tokens.ErrTxWithWrongValue
			}
		}

		assetAmount := receiverAssetsMap[multichainToken]
		if value, err := common.GetBigIntFromStr(assetAmount); err != nil {
			return err
		} else {
			if value.Cmp(args.OriginValue) > 0 {
				return tokens.ErrTxWithWrongValue
			}
		}
	default:
		return tokens.ErrTxWithWrongAssetLength
	}
	return nil
}

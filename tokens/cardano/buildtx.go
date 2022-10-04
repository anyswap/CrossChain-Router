package cardano

import (
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
		if _, err := b.initExtra(args); err != nil {
			return nil, err
		} else {
			if utxos, err := b.QueryUtxo(routerMPC); err != nil {
				return nil, err
			} else {
				swapId := fmt.Sprintf("%s-%d", args.SwapID, args.LogIndex)
				if rawTransaction, err := b.BuildTx(swapId, receiver, multichainToken, amount, utxos); err != nil {
					return nil, err
				} else {
					if err := CreateRawTx(rawTransaction); err != nil {
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
											if err := CreateRawTx(rawTransaction); err != nil {
												return nil, err
											} else {
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

func (b *Bridge) BuildTx(swapId, receiver, asset string, amount *big.Int, utxos map[OutputKey]UtxoMap) (*RawTransaction, error) {
	log.Infof("build Tx: receiver:%+v asset:%+v amount:%+v", receiver, asset, amount)
	routerMpc := b.GetRouterContract("")
	rawTransaction := &RawTransaction{
		Fee:     "0",
		OutFile: swapId,
		TxOuts:  map[string]map[string]string{},
		TxInts:  map[string]string{},
	}
	allAssetsMap := map[string]uint64{}

	targetAsset := &TargetAsset{
		AdaFinish: false,
	}

	if asset == AdaAsset {
		targetAsset.TokenFinish = true
		targetAsset.NeedAdaValue = amount.Uint64()
	} else {
		targetAsset.TokenFinish = false
		targetAsset.NeedAdaValue = FixAdaAmount.Uint64()
	}

	for outputKey, utxoInfo := range utxos {
		if targetAsset.TokenFinish || utxoInfo.Assets[asset] != "" {
			rawTransaction.TxInts[outputKey.TxHash] = fmt.Sprint(outputKey.Index)
			for asset, assetAmount := range utxoInfo.Assets {
				if value, err := common.GetBigIntFromStr(assetAmount); err != nil {
					return nil, err
				} else {
					allAssetsMap[asset] += value.Uint64()
				}
			}
			if allAssetsMap[asset] >= amount.Uint64() {
				targetAsset.TokenFinish = true
			}
			if allAssetsMap[AdaAsset] >= targetAsset.NeedAdaValue+DefaultAdaAmount.Uint64() {
				targetAsset.AdaFinish = true
			}
			if targetAsset.TokenFinish && targetAsset.AdaFinish {
				break
			}
		}
	}

	if !targetAsset.AdaFinish {
		return nil, tokens.ErrAdaBalancesNotEnough
	}

	rawTransaction.TxOuts[receiver] = map[string]string{}
	rawTransaction.TxOuts[routerMpc] = map[string]string{}
	if asset != AdaAsset {
		policyId := strings.Split(asset, ".")[0]
		if targetAsset.TokenFinish {
			rawTransaction.TxOuts[receiver][asset] = amount.String()
			if allAssetsMap[asset] > amount.Uint64() {
				if policyId != MPCPolicyId {
					rawTransaction.TxOuts[routerMpc][asset] = fmt.Sprint((allAssetsMap[asset] - amount.Uint64()))
				} else {
					rawTransaction.Mint = map[string]string{
						asset: fmt.Sprintf("-%s", fmt.Sprint(allAssetsMap[asset]-amount.Uint64())),
					}
				}
			}
		} else {
			if policyId != MPCPolicyId {
				return nil, tokens.ErrTokenBalancesNotEnough
			} else {
				rawTransaction.Mint = map[string]string{
					asset: fmt.Sprint(amount.Uint64() - allAssetsMap[asset]),
				}
				rawTransaction.TxOuts[receiver][asset] = amount.String()
			}
		}
	}

	rawTransaction.TxOuts[receiver][AdaAsset] = fmt.Sprint(targetAsset.NeedAdaValue)
	rawTransaction.TxOuts[routerMpc][AdaAsset] = fmt.Sprint((allAssetsMap[AdaAsset] - targetAsset.NeedAdaValue))

	for otherAsset, assetAmount := range allAssetsMap {
		if otherAsset != AdaAsset && otherAsset != asset {
			rawTransaction.TxOuts[routerMpc][otherAsset] = fmt.Sprint(assetAmount)
		}
	}
	return rawTransaction, nil
}

func CreateRawTx(rawTransaction *RawTransaction) error {
	cmdString := ""
	inputString := ""
	for txHash, index := range rawTransaction.TxInts {
		inputString = fmt.Sprintf("%s  --tx-in  %s#%s", inputString, txHash, index)
	}
	outputString := ""
	for address, assets := range rawTransaction.TxOuts {
		outputString = fmt.Sprintf("%s  --tx-out  %s+%s", outputString, address, assets[AdaAsset])
		for asset, amount := range assets {
			if asset != AdaAsset {
				outputString = fmt.Sprintf("%s+%s %s", outputString, amount, asset)
			}
		}
	}
	if rawTransaction.Mint != nil {
		mintString := ""
		for asset, amount := range rawTransaction.Mint {
			mintString = fmt.Sprintf("%s  --mint=%s %s", mintString, amount, asset)
		}
		cmdString = fmt.Sprintf(BuildRawTxWithMintCmd, rawTransaction.Fee, inputString, outputString, mintString, RawPath+rawTransaction.OutFile+RawSuffix)
	} else {
		cmdString = fmt.Sprintf(BuildRawTxWithoutMintCmd, rawTransaction.Fee, inputString, outputString, RawPath+rawTransaction.OutFile+RawSuffix)
	}
	if _, err := ExecCmd(cmdString, "  "); err != nil {
		return err
	}
	return nil
}

func CalcMinFee(rawTransaction *RawTransaction) (string, error) {
	txBodyPath := RawPath + rawTransaction.OutFile + RawSuffix
	cmdString := fmt.Sprintf(CalcMinFeeCmd, txBodyPath, len(rawTransaction.TxInts), len(rawTransaction.TxOuts))
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

func (b *Bridge) QueryUtxo(address string) (map[OutputKey]UtxoMap, error) {
	utxos := make(map[OutputKey]UtxoMap)
	if outputs, err := b.GetOutputsByAddress(address); err != nil {
		return nil, err
	} else {
		for _, output := range *outputs {
			outputKey := OutputKey{TxHash: output.TxHash, Index: output.Index}
			utxos[outputKey] = UtxoMap{
				Assets: make(map[string]string),
			}
			utxos[outputKey].Assets[AdaAsset] = output.Value
			for _, token := range output.Tokens {
				utxos[outputKey].Assets[token.Asset.PolicyId+"."+token.Asset.AssetName] = token.Quantity
			}
		}
		return utxos, nil
	}
}

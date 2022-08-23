package cardano

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"os/exec"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	RawPath    = "txDb/raw/"
	AdaAssetId = "lovelace"
	RawSuffix  = ".raw"
)

var (
	FixAdaAmount             = big.NewInt(1000000)
	BuildRawTxWithoutMintCmd = "cardano-cli  transaction  build-raw  --fee  %s%s%s  --out-file  %s"
	CalcMinFeeCmd            = "cardano-cli transaction calculate-min-fee --tx-body-file %s --tx-in-count %d --tx-out-count %d --witness-count 1 --testnet-magic 1097911063 --protocol-params-file txDb/config/protocol.json"
	QueryUtxo                = "cardano-cli query utxo --address %s --testnet-magic %s"
	CalcTxIdCmd              = "cardano-cli transaction txid --tx-body-file %s"
)

// BuildRawTransaction build raw tx
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
			if utxos, err := b.queryUtxoCmd(routerMPC); err != nil {
				return nil, err
			} else {
				swapId := fmt.Sprintf("%s-%d", args.SwapID, args.LogIndex)
				if rawTransaction, err := b.buildTx(swapId, receiver, multichainToken, amount.String(), utxos); err != nil {
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

func (b *Bridge) buildTx(swapId, receiver, assetId, amount string, utxos map[string]UtxoMap) (*RawTransaction, error) {
	log.Infof("build Tx:\nreceiver:%+v\nassetId:%+v\namount:%+v\nutxos:%+v", receiver, assetId, amount, utxos)
	routerMpc := b.GetRouterContract("")
	rawTransaction := RawTransaction{
		Fee:     "0",
		OutFile: swapId,
		TxOuts:  map[string]map[string]string{},
		TxInts:  map[string]string{},
	}
	for txHash, utxoInfo := range utxos {
		if utxoInfo.Assets[assetId] != "" {
			if value, err := common.GetBigIntFromStr(utxoInfo.Assets[assetId]); err != nil {
				return nil, err
			} else {
				if amountValue, err := common.GetBigIntFromStr(amount); err != nil {
					return nil, err
				} else {
					if value.Cmp(amountValue) >= 0 {
						rawTransaction.TxInts[txHash] = utxoInfo.Index
						if rawTransaction.TxOuts[receiver] == nil {
							rawTransaction.TxOuts[receiver] = map[string]string{}
						}
						rawTransaction.TxOuts[receiver][assetId] = amountValue.String()
						rawTransaction.TxOuts[receiver][AdaAssetId] = FixAdaAmount.String()
						if adaAmount, err := common.GetBigIntFromStr(utxoInfo.Assets[AdaAssetId]); err != nil {
							return nil, err
						} else {
							if rawTransaction.TxOuts[routerMpc] == nil {
								rawTransaction.TxOuts[routerMpc] = map[string]string{}
							}
							rawTransaction.TxOuts[routerMpc][AdaAssetId] = adaAmount.Sub(adaAmount, FixAdaAmount).String()
						}
						rawTransaction.TxOuts[routerMpc][assetId] = value.Sub(value, amountValue).String()
						return &rawTransaction, nil
					}
				}
			}
		}
	}
	return nil, errors.New("build tx fails,output not match asset")
}

func CreateRawTx(rawTransaction *RawTransaction) error {
	inputString := ""
	for txHash, index := range rawTransaction.TxInts {
		inputString = fmt.Sprintf("%s  --tx-in  %s#%s", inputString, txHash, index)
	}
	outputString := ""
	for address, assets := range rawTransaction.TxOuts {
		outputString = fmt.Sprintf("%s  --tx-out  %s+%s", outputString, address, assets[AdaAssetId])
		for assetId, amount := range assets {
			if assetId != AdaAssetId {
				outputString = fmt.Sprintf("%s+%s %s", outputString, amount, assetId)
			}
		}
	}
	cmdString := fmt.Sprintf(BuildRawTxWithoutMintCmd, rawTransaction.Fee, inputString, outputString, RawPath+rawTransaction.OutFile+RawSuffix)
	list := strings.Split(cmdString, "  ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func CalcMinFee(rawTransaction *RawTransaction) (string, error) {
	txBodyPath := RawPath + rawTransaction.OutFile + RawSuffix
	cmdString := fmt.Sprintf(CalcMinFeeCmd, txBodyPath, len(rawTransaction.TxInts), len(rawTransaction.TxOuts))
	list := strings.Split(cmdString, " ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return cmdOut.String(), nil
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
	return 0, tokens.ErrNotImplemented
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

func (b *Bridge) queryUtxoCmd(address string) (map[string]UtxoMap, error) {
	utxos := make(map[string]UtxoMap)
	list := strings.Split(fmt.Sprintf(QueryUtxo, address, b.ChainConfig.ChainID), " ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		return nil, err
	} else {
		res := cmdOut.String()
		if list := strings.Split(res, "--------------------------------------------------------------------------------------"); len(list) != 2 {
			return nil, errors.New("queryUtxo length not match")
		} else {
			if outputList := strings.Split(list[1], "\n"); len(outputList) < 3 {
				return nil, errors.New("outputList length not match")
			} else {
				for _, output := range outputList[1 : len(outputList)-1] {
					if assetsInfoList := strings.Split(output, "        "); len(assetsInfoList) != 2 {
						return nil, errors.New("assetsInfoList length not match")
					} else {
						if txAndIndex := strings.Split(assetsInfoList[0], "     "); len(txAndIndex) != 2 {
							return nil, errors.New("txAndIndex length not match")
						} else {
							utxos[txAndIndex[0]] = UtxoMap{
								Index:  txAndIndex[1],
								Assets: make(map[string]string),
							}
							if assetAndAmountList := strings.Split(assetsInfoList[1], " + "); len(assetAndAmountList) < 2 {
								return nil, errors.New("assetAndAmountList length not match")
							} else {
								for _, assetAndAmount := range assetAndAmountList[:len(assetAndAmountList)-1] {
									if assetAmount := strings.Split(assetAndAmount, " "); len(assetAmount) != 2 {
										return nil, errors.New("assetAmount length not match")
									} else {
										utxos[txAndIndex[0]].Assets[assetAmount[1]] = assetAmount[0]
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return utxos, nil
}

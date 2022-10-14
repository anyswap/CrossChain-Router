package cardano

// import (
// 	"fmt"
// 	"strings"
// 	"time"

// 	"github.com/anyswap/CrossChain-Router/v3/common"
// 	"github.com/anyswap/CrossChain-Router/v3/log"
// 	"github.com/anyswap/CrossChain-Router/v3/tokens"
// )

// func (b *Bridge) BuildAggregateTx(swapId string, utxos map[UtxoKey]AssetsMap) (*RawTransaction, error) {
// 	log.Infof("BuildAggregateTx:\nswapId:%+v\nutxos:%+v\n", swapId, utxos)
// 	routerMpc := b.GetRouterContract("")
// 	rawTransaction := &RawTransaction{
// 		Fee:     "0",
// 		OutFile: swapId,
// 		TxOuts:  make(map[string]AssetsMap),
// 		TxIns:   []UtxoKey{},
// 	}
// 	allAssetsMap := map[string]uint64{}
// 	for utxoKey, assetsMap := range utxos {
// 		rawTransaction.TxIns = append(rawTransaction.TxIns, utxoKey)
// 		for asset, assetAmount := range assetsMap {
// 			if value, err := common.GetBigIntFromStr(assetAmount); err != nil {
// 				return nil, err
// 			} else {
// 				allAssetsMap[asset] += value.Uint64()
// 			}
// 		}
// 	}

// 	rawTransaction.TxOuts[routerMpc] = map[string]string{}
// 	for assetIdWithName, assetAmount := range allAssetsMap {
// 		policyId := strings.Split(assetIdWithName, ".")[0]
// 		if policyId != MPCPolicyId {
// 			rawTransaction.TxOuts[routerMpc][assetIdWithName] = fmt.Sprint(assetAmount)
// 		} else {
// 			rawTransaction.Mint = map[string]string{
// 				assetIdWithName: fmt.Sprintf("-%d", assetAmount),
// 			}
// 		}
// 	}
// 	return rawTransaction, nil
// }

// func (b *Bridge) SignAggregateTx(swapId string, rawTx interface{}) (string, error) {
// 	if rawTransaction, ok := rawTx.(*RawTransaction); !ok {
// 		return "", tokens.ErrWrongRawTx
// 	} else {
// 		mpcAddress := b.GetRouterContract("")
// 		args := &tokens.BuildTxArgs{
// 			SwapArgs: tokens.SwapArgs{
// 				Identifier: tokens.AggregateIdentifier,
// 			},
// 			From: mpcAddress,
// 		}
// 		if signTx, _, err := b.MPCSignTransaction(rawTransaction, args); err != nil {
// 			return "", err
// 		} else {
// 			if txHash, err := b.SendTransaction(signTx); err != nil {
// 				return "", err
// 			} else {
// 				return txHash, nil
// 			}
// 		}
// 	}
// }

// func (b *Bridge) AggregateTx() (txHash string, err error) {
// 	mpcAddress := b.GetRouterContract("")
// 	swapId := fmt.Sprintf("doAggregateJob_%s", time.Now())
// 	if utxo, err := b.QueryUtxoOnChain(mpcAddress); err == nil {
// 		if rawTransaction, err := b.BuildAggregateTx(swapId, utxo); err == nil {
// 			if err := CreateRawTx(rawTransaction, mpcAddress); err == nil {
// 				if minFee, err := CalcMinFee(rawTransaction); err == nil {
// 					if feeList := strings.Split(minFee, " "); len(feeList) == 2 {
// 						rawTransaction.Fee = feeList[0]
// 						if adaAmount, err := common.GetBigIntFromStr(rawTransaction.TxOuts[mpcAddress][AdaAsset]); err == nil {
// 							if feeAmount, err := common.GetBigIntFromStr(feeList[0]); err == nil {
// 								returnAmount := adaAmount.Sub(adaAmount, feeAmount)
// 								if returnAmount.Cmp(FixAdaAmount) > 0 {
// 									rawTransaction.TxOuts[mpcAddress][AdaAsset] = returnAmount.String()
// 									if err := CreateRawTx(rawTransaction, mpcAddress); err == nil {
// 										if txHash, err := b.SignAggregateTx(swapId, rawTransaction); err == nil {
// 											return txHash, nil
// 										}
// 									}
// 								}
// 							}
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return "", err
// }

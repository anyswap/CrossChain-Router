package main

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"os/exec"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
)

// import (
// 	"fmt"
// 	"time"

// 	"github.com/anyswap/CrossChain-Router/v3/log"
// 	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
// 	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
// )

// const (
// 	rpcTimeout  = 1111111111111111111
// 	url         = "https://graphql-api.testnet.dandelion.link/"
// 	queryMethod = "{transactions(where: { hash: { _eq: \"%s\"}}) {block {number epochNo}hash metadata{key value}inputs{tokens{asset{ assetId assetName}quantity }value}outputs{address tokens{ asset{assetId assetName}quantity}value}validContract}}"
// )

const (
	RawPath    = "txDb/raw/"
	AdaAssetId = "lovelace"
)

var (
	FixAdaAmount = big.NewInt(1000000)
	swapId       = "d72bdd41f7e6060a1af3761aafc2d6113780f40616967317e54fdff91d148e97-0"
	// txHash   = "d72bdd41f7e6060a1af3761aafc2d6113780f40616967317e54fdff91d148e97"
	sender   = "addr_test1vrxa4lr0ejqd8ze46ejft0646h6j4kxh56g7feumntq78nq89mfmy"
	receiver = "addr_test1vqvlku0ytscqg32rpv660uu4sgxlje25s5xrpz7zjqsva3c8pfckz"
	assetId  = "f3f97a8f8af955089c1865de77f37d97cbaf4918fb19ce7b3718f3bd.55534454"
	amount   = "12345678"
	ZeroFee  = "0"
	// queryTip string = "cardano-cli query tip --testnet-magic 1097911063"
	BuildRawTxWithoutMintCmd = "cardano-cli  transaction  build-raw  --fee  %s%s%s  --out-file  %s"
	// buildRawTxWithMintCmd           = "cardano-cli  transaction  build-raw  --fee  %s%s%s  --mint=%s  --out-file  %s"
	CalcMinFeeCmd        = "cardano-cli transaction calculate-min-fee --tx-body-file %s --tx-in-count %d --tx-out-count %d --witness-count 1 --testnet-magic 1097911063 --protocol-params-file txDb/config/protocol.json"
	QueryUtxo     string = "cardano-cli query utxo --address %s --testnet-magic 1097911063"
)

func main() {
	// log.SetLogger(6, false, true)

	// queryTipCmd()
	// queryTx(txHash)

	utxos := queryUtxoCmd()
	log.Infof("queryUtxoCmd %+v", utxos)

	// build tx
	if rawTransaction, err := buildTx(swapId, receiver, assetId, amount, ZeroFee, utxos); err != nil {
		log.Fatal("buildTx fails", "err", err)
	} else {
		log.Info("\nbuildTx success", "rawTransaction", rawTransaction)
		if txPath, err := createRawTx(rawTransaction); err != nil {
			log.Fatal("createRawTx fails", "err", err)
		} else {
			if minFee, err := calcMinFee(rawTransaction, txPath); err != nil {
				log.Fatal("calcMinFee fails", "err", err)
			} else {
				if feeList := strings.Split(minFee, " "); len(feeList) != 2 {
					log.Fatal("feeList length not match", "want", 2, "get", len(feeList))
				} else {
					rawTransaction.Fee = feeList[0]
					if feeTxPath, err := createRawTx(rawTransaction); err != nil {
						log.Fatal("createRawTx fails", "err", err)
					} else {
						log.Info("create fee Tx", "feeTxPath", feeTxPath)
					}
				}
			}
		}
	}
}

func calcMinFee(rawTransaction *cardano.RawTransaction, rawPath string) (string, error) {
	if RawPath+rawTransaction.OutFile+".raw" != rawPath {
		return "", errors.New("raw path not match")
	}
	cmdString := fmt.Sprintf(CalcMinFeeCmd, rawPath, len(rawTransaction.TxInts), len(rawTransaction.TxOuts))
	log.Info("cmdString", "cmdString", cmdString)
	list := strings.Split(cmdString, " ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		log.Fatal("fails", "cmdErr", cmdErr.String())
	}
	return cmdOut.String(), nil
}

func createRawTx(rawTransaction *cardano.RawTransaction) (string, error) {
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
	cmdString := fmt.Sprintf(BuildRawTxWithoutMintCmd, rawTransaction.Fee, inputString, outputString, RawPath+rawTransaction.OutFile+".raw")
	log.Info("cmdString", "cmdString", cmdString)
	list := strings.Split(cmdString, "  ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		log.Fatal("fails", "cmdErr", cmdErr.String())
	} else {
		log.Info("success", "cmdOut", cmdOut.String())
	}
	return RawPath + rawTransaction.OutFile + ".raw", nil
}

func buildTx(swapId, receiver, assetId, amount, fee string, utxos map[string]cardano.UtxoMap) (*cardano.RawTransaction, error) {
	log.Infof("build Tx:\nreceiver:%+v\nassetId:%+v\namount:%+v\nutxos:%+v", receiver, assetId, amount, utxos)
	rawTransaction := cardano.RawTransaction{
		Fee:     fee,
		OutFile: swapId,
		TxOuts:  map[string]map[string]string{},
		TxInts:  map[string]string{},
	}
	for txHash, utxoInfo := range utxos {
		if utxoInfo.Assets[assetId] != "" {
			log.Infof("\nassetId:%+v amount:%+v", assetId, utxoInfo.Assets[assetId])
			if value, err := common.GetBigIntFromStr(utxoInfo.Assets[assetId]); err != nil {
				log.Fatal("GetBigIntFromStr error", "err", err)
			} else {
				if amountValue, err := common.GetBigIntFromStr(amount); err != nil {
					log.Fatal("GetBigIntFromStr error", "err", err)
				} else {
					if value.Cmp(amountValue) >= 0 {
						rawTransaction.TxInts[txHash] = utxoInfo.Index
						if rawTransaction.TxOuts[receiver] == nil {
							rawTransaction.TxOuts[receiver] = map[string]string{}
						}
						rawTransaction.TxOuts[receiver][assetId] = amountValue.String()
						rawTransaction.TxOuts[receiver][AdaAssetId] = FixAdaAmount.String()
						if adaAmount, err := common.GetBigIntFromStr(utxoInfo.Assets[AdaAssetId]); err != nil {
							log.Fatal("GetBigIntFromStr error", "err", err)
						} else {
							if rawTransaction.TxOuts[sender] == nil {
								rawTransaction.TxOuts[sender] = map[string]string{}
							}
							rawTransaction.TxOuts[sender][AdaAssetId] = adaAmount.Sub(adaAmount, FixAdaAmount).String()
						}
						rawTransaction.TxOuts[sender][assetId] = value.Sub(value, amountValue).String()
					}
				}
			}
		}
	}
	return &rawTransaction, nil
}

// func queryTx(txHash string) (*cardano.TransactionResult, error) {
// 	request := &client.Request{}
// 	request.Params = fmt.Sprintf(queryMethod, txHash)
// 	request.ID = int(time.Now().UnixNano())
// 	request.Timeout = rpcTimeout
// 	var result cardano.TransactionResult
// 	err := client.CardanoPostRequest(url, request, &result)
// 	if err != nil {
// 		log.Fatal("queryTx error", "txHash", txHash)
// 		return nil, err
// 	}
// 	log.Infof("queryTx success:%+v", result)
// 	return &result, nil
// }

// func queryTipCmd() {
// 	list := strings.Split(queryTip, " ")
// 	cmd := exec.Command(list[0], list[1:]...)
// 	var cmdOut bytes.Buffer
// 	var cmdErr bytes.Buffer
// 	cmd.Stdout = &cmdOut
// 	cmd.Stderr = &cmdErr
// 	if err := cmd.Run(); err != nil {
// 		log.Fatal("fails", "cmdErr", cmdErr.String())
// 	} else {
// 		var tip cardano.Tip
// 		if err := json.Unmarshal(cmdOut.Bytes(), &tip); err == nil {
// 			log.Info("success", "tip", tip)
// 		}
// 	}
// }

func queryUtxoCmd() map[string]cardano.UtxoMap {
	utxos := make(map[string]cardano.UtxoMap)
	list := strings.Split(fmt.Sprintf(QueryUtxo, sender), " ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		log.Fatal("fails", "cmdErr", cmdErr.String())
	} else {
		res := cmdOut.String()
		if list := strings.Split(res, "--------------------------------------------------------------------------------------"); len(list) != 2 {
			log.Fatal("queryUtxo fails", "len", len(list))
		} else {
			if outputList := strings.Split(list[1], "\n"); len(outputList) < 3 {
				log.Fatal("outputList length is zero", "outputList", outputList)
			} else {
				for _, output := range outputList[1 : len(outputList)-1] {
					if assetsInfoList := strings.Split(output, "        "); len(assetsInfoList) != 2 {
						log.Fatal("assetsInfoList length err", "want", 2, "get", len(assetsInfoList))
					} else {
						if txAndIndex := strings.Split(assetsInfoList[0], "     "); len(txAndIndex) != 2 {
							log.Fatal("txAndIndex length err", "want", 2, "get", len(txAndIndex))
						} else {
							utxos[txAndIndex[0]] = cardano.UtxoMap{
								Index:  txAndIndex[1],
								Assets: make(map[string]string),
							}
							if assetAndAmountList := strings.Split(assetsInfoList[1], " + "); len(assetAndAmountList) < 2 {
								log.Fatal("assetAndAmountList length err", "min", 2, "get", len(assetAndAmountList))
							} else {
								for _, assetAndAmount := range assetAndAmountList[:len(assetAndAmountList)-1] {
									if assetAmount := strings.Split(assetAndAmount, " "); len(assetAmount) != 2 {
										log.Fatal("assetAmount length err", "want", 2, "get", len(assetAmount))
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
	return utxos
}

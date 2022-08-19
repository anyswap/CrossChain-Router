package main

import (
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
)

const (
	rpcTimeout  = 1111111111111111111
	url         = "https://graphql-api.testnet.dandelion.link/"
	queryMethod = "{transactions(where: { hash: { _eq: \"%s\"}}) {block {number epochNo}hash metadata{key value}inputs{tokens{asset{ assetId assetName}quantity }value}outputs{address tokens{ asset{assetId assetName}quantity}value}validContract}}"
)

var (
	txHash = "d72bdd41f7e6060a1af3761aafc2d6113780f40616967317e54fdff91d148e97"

// queryTip  string = "cardano-cli query tip --testnet-magic 1097911063"
// queryUtxo string = "cardano-cli query utxo --address addr_test1vrxa4lr0ejqd8ze46ejft0646h6j4kxh56g7feumntq78nq89mfmy --testnet-magic 1097911063"
)

func main() {
	log.SetLogger(6, false, true)

	// queryTipCmd()
	// queryUtxoCmd()
	queryTx(txHash)
}

func queryTx(txHash string) (*cardano.TransactionResult, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(queryMethod, txHash)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result cardano.TransactionResult
	err := client.CardanoPostRequest(url, request, &result)
	if err != nil {
		log.Fatal("queryTx error", "txHash", txHash)
		return nil, err
	}
	log.Infof("queryTx success:%+v", result)
	return &result, nil
}

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

// func queryUtxoCmd() {
// 	list := strings.Split(queryUtxo, " ")
// 	cmd := exec.Command(list[0], list[1:]...)
// 	var cmdOut bytes.Buffer
// 	var cmdErr bytes.Buffer
// 	cmd.Stdout = &cmdOut
// 	cmd.Stderr = &cmdErr
// 	if err := cmd.Run(); err != nil {
// 		log.Fatal("fails", "cmdErr", cmdErr.String())
// 	} else {
// 		res := cmdOut.String()
// 		if list := strings.Split(res, "--------------------------------------------------------------------------------------"); len(list) != 2 {
// 			log.Fatal("queryUtxo fails", "len", len(list))
// 		} else {
// 			if outputList := strings.Split(list[1], "\n"); len(outputList) < 3 {
// 				log.Fatal("outputList length is zero", "outputList", outputList)
// 			} else {
// 				for _, output := range outputList[1 : len(outputList)-1] {
// 					if assetsInfoList := strings.Split(output, "        "); len(assetsInfoList) != 2 {
// 						log.Fatal("assetsInfoList length err", "want", 2, "get", len(assetsInfoList))
// 					} else {
// 						if txAndIndex := strings.Split(assetsInfoList[0], "     "); len(txAndIndex) != 2 {
// 							log.Fatal("txAndIndex length err", "want", 2, "get", len(txAndIndex))
// 						} else {
// 							for _, txIndex := range txAndIndex {
// 								log.Info("txAndIndex", "txIndex", txIndex)
// 							}
// 						}
// 						if assetAndAmountList := strings.Split(assetsInfoList[1], " + "); len(assetAndAmountList) < 2 {
// 							log.Fatal("assetAndAmountList length err", "min", 2, "get", len(assetAndAmountList))
// 						} else {
// 							for _, assetAndAmount := range assetAndAmountList[:len(assetAndAmountList)-1] {
// 								if assetAmount := strings.Split(assetAndAmount, " "); len(assetAmount) != 2 {
// 									log.Fatal("assetAmount length err", "want", 2, "get", len(assetAmount))
// 								} else {
// 									log.Info("assetAndAmount", "assetAmount", assetAmount)
// 								}
// 							}
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// }

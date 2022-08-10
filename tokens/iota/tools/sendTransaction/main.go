package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/iota"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	paramNetwork string
	paramPubKey  string
	paramPrivKey string
	paramTo      string
	paramIndex   string
	paramData    string
	paramValue   string
)

func initFlags() {
	flag.StringVar(&paramNetwork, "n", "", "network url")
	flag.StringVar(&paramPubKey, "publicKey", "", "sign public key")
	flag.StringVar(&paramPrivKey, "privKey", "", "sign privKey")
	flag.StringVar(&paramTo, "to", "", "target addr")
	flag.StringVar(&paramIndex, "index", "", "payload index")
	flag.StringVar(&paramData, "data", "", "payload data")
	flag.StringVar(&paramValue, "value", "", "send value")
	flag.Parse()
}

var (
	returnValue uint64 = 0
	tempValue   uint64 = 0
)

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	// create a new node API client
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(paramNetwork)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()

	_, toEdAddr, err := iotago.ParseBech32(paramTo)
	if err != nil {
		log.Fatal("ParseBech32", "paramTo", paramTo, "err", err)
	}

	publicKey, err := hex.DecodeString(paramPubKey)
	if err != nil {
		log.Fatal("DecodeString", "paramPubKey", paramPubKey, "err", err)
	}
	edAddr := iotago.AddressFromEd25519PubKey(publicKey)

	needAmount, err := strconv.ParseUint(paramValue, 10, 64)
	if err != nil {
		log.Fatal("ParseUint", "paramValue", paramValue, "err", err)
	}
	if balance, err := iota.CheckBalance(paramNetwork, &edAddr, needAmount); err != nil {
		log.Fatal("CheckBalance", "balance", balance, "needAmount", needAmount, "err", err)
	}
	var inputs []*iotago.ToBeSignedUTXOInput
	var outputs []*iotago.SigLockedSingleOutput
	// fetch the node's info to know the min. required PoW score

	if outPutIDs, err := iota.GetOutPutIDs(paramNetwork, &edAddr); err != nil {
		log.Fatal("GetOutPutIDs", "paramNetwork", paramNetwork, "edAddr", edAddr)
	} else {
		value := needAmount
		finish := false
		for _, outputID := range outPutIDs {
			if outPut, needValue, returnValue, err := iota.GetOutPutByID(paramNetwork, outputID.MustAsUTXOInput().ID(), value, finish); err == nil {
				inputs = append(inputs, &iotago.ToBeSignedUTXOInput{Address: &edAddr, Input: outPut})
				if needValue == 0 {
					if returnValue == 0 || returnValue >= iota.KeepAlive {
						outputs = append(outputs, &iotago.SigLockedSingleOutput{Address: toEdAddr, Amount: needAmount})
						if returnValue != 0 {
							outputs = append(outputs, &iotago.SigLockedSingleOutput{Address: &edAddr, Amount: returnValue})
						}
						break
					} else {
						value = returnValue
						finish = true
					}
				} else {
					value = needValue
				}
			}
		}
		indexationPayload := &iotago.Indexation{
			Index: []byte(paramIndex),
			Data:  []byte(paramData),
		}

		if messageBuilder := iota.BuildMessage(inputs, outputs, indexationPayload); messageBuilder == nil {
			log.Fatal("BuildMessage", "inputs", inputs, "outputs", outputs, "indexationPayload", indexationPayload)
		} else {
			if paramPrivKey != "" {
				priv, _ := hex.DecodeString(paramPrivKey)
				signKey := iotago.NewAddressKeysForEd25519Address(&edAddr, priv)
				signer := iotago.NewInMemoryAddressSigner(signKey)
				if message, err := iota.ProofOfWork(paramNetwork, messageBuilder.TransactionBuilder.
					BuildAndSwapToMessageBuilder(signer, nil)); err != nil {
					log.Fatal("NewTransactionBuilder", "err", err)
				} else {
					if res, err := nodeHTTPAPIClient.SubmitMessage(ctx, message); err != nil {
						log.Fatal("SubmitMessage", "err", err)
					} else {
						fmt.Printf("res: %+v\n", iotago.MessageIDToHexString(res.MustID()))
					}
				}
			} else {
				log.Fatal("not support mpc sign now")
			}
		}
	}
}

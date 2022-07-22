package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/iota"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	paramNetwork  string
	paramPubKey   string
	paramPrivKey  string
	paramToPubKey string
	paramIndex    string
	paramData     string
	paramValue    string
)

func initFlags() {
	flag.StringVar(&paramNetwork, "n", "", "network url")
	flag.StringVar(&paramPubKey, "publicKey", "", "sign public key")
	flag.StringVar(&paramPrivKey, "privKey", "", "sign privKey")
	flag.StringVar(&paramToPubKey, "to", "", "target public key")
	flag.StringVar(&paramIndex, "index", "", "payload index")
	flag.StringVar(&paramData, "data", "", "payload data")
	flag.StringVar(&paramValue, "value", "", "send value")
	flag.Parse()
}

var (
	returnValue uint64 = 0
)

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	// create a new node API client
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(paramNetwork)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()

	// fetch the node's info to know the min. required PoW score
	if info, err := nodeHTTPAPIClient.Info(ctx); err != nil {
		log.Fatal("Info", "paramNetwork", paramNetwork, "err", err)
	} else {
		fmt.Printf("info: %+v\n", info)

		needValue, err := strconv.ParseUint(paramValue, 10, 64)
		if err != nil {
			log.Fatal("ParseUint", "paramValue", paramValue, "err", err)
		}
		if publicKey, err := hex.DecodeString(paramPubKey); err != nil {
			log.Fatal("DecodeString", "paramPubKey", paramPubKey, "err", err)
		} else {
			// craft an indexation payload
			indexationPayload := &iotago.Indexation{
				Index: []byte(paramIndex),
				Data:  []byte(paramData),
			}
			edAddr := iotago.AddressFromEd25519PubKey(publicKey)
			priv, _ := hex.DecodeString(paramPrivKey)
			bech32Addr := edAddr.Bech32(iotago.NetworkPrefix(info.Bech32HRP))
			fmt.Printf("edAddr: %+v\niotaAddr: %+v\n", edAddr.String(), bech32Addr)

			signKey := iotago.NewAddressKeysForEd25519Address(&edAddr, priv)
			signer := iotago.NewInMemoryAddressSigner(signKey)

			var inputUTXO1 iotago.UTXOInput
			outputResponse, _, err := nodeHTTPAPIClient.OutputsByEd25519Address(ctx, &edAddr, false)
			if err != nil {
				log.Fatal("OutputsByEd25519Address", "edAddr", edAddr, "err", err)
			}
			for _, outputID := range outputResponse.OutputIDs {
				if outputRes, err := nodeHTTPAPIClient.OutputByID(ctx, outputID.MustAsUTXOInput().ID()); err != nil {
					log.Fatal("OutputByID", "OutputID", outputID.MustAsUTXOInput().ID(), "err", err)
				} else {
					var rawType iota.RawType
					rawOutPut, _ := outputRes.RawOutput.MarshalJSON()
					err := json.Unmarshal(rawOutPut, &rawType)
					if err != nil {
						log.Fatal("Unmarshal", "rawOutPut", rawOutPut, "err", err)
					}
					if rawType.Amount > needValue {
						returnValue = rawType.Amount - needValue
						transactionID, _ := hex.DecodeString(outputRes.TransactionID)
						copy(inputUTXO1.TransactionID[:], transactionID)
						inputUTXO1.TransactionOutputIndex = outputRes.OutputIndex
						break
					}
				}
			}

			toPublicKey, err := hex.DecodeString(paramToPubKey)
			if err != nil {
				log.Fatal("DecodeString", "paramToPubKey", paramToPubKey, "err", err)
			}
			toEdAddr := iotago.AddressFromEd25519PubKey(toPublicKey)

			if message, err := iotago.NewTransactionBuilder().
				AddInput(&iotago.ToBeSignedUTXOInput{Address: &edAddr, Input: &inputUTXO1}).
				AddOutput(&iotago.SigLockedSingleOutput{Address: &toEdAddr, Amount: needValue}).
				AddOutput(&iotago.SigLockedSingleOutput{Address: &edAddr, Amount: returnValue}).
				AddIndexationPayload(indexationPayload).BuildAndSwapToMessageBuilder(signer, nil).Build(); err != nil {
				log.Fatal("NewTransactionBuilder", "err", err)
			} else {
				if res, err := nodeHTTPAPIClient.SubmitMessage(ctx, message); err != nil {
					log.Fatal("SubmitMessage", "err", err)
				} else {
					fmt.Printf("res: %+v\n", res)
				}
			}
		}
	}
}

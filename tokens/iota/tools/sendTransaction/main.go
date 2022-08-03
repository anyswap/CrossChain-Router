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

	needValue, err := strconv.ParseUint(paramValue, 10, 64)
	if err != nil {
		log.Fatal("ParseUint", "paramValue", paramValue, "err", err)
	}
	if balance, err := nodeHTTPAPIClient.BalanceByEd25519Address(ctx, &edAddr); err != nil || balance.Balance < needValue {
		log.Fatal("BalanceByEd25519Address", "balance", balance.Balance, "needValue", needValue, "err", err)
	}

	// fetch the node's info to know the min. required PoW score
	if info, err := nodeHTTPAPIClient.Info(ctx); err != nil {
		log.Fatal("Info", "paramNetwork", paramNetwork, "err", err)
	} else {
		fmt.Printf("info: %+v\n", info)
		// craft an indexation payload
		indexationPayload := &iotago.Indexation{
			Index: []byte(paramIndex),
			Data:  []byte(paramData),
		}

		outputResponse, _, err := nodeHTTPAPIClient.OutputsByEd25519Address(ctx, &edAddr, false)
		if err != nil {
			log.Fatal("OutputsByEd25519Address", "edAddr", edAddr, "err", err)
		}

		var inputs []*iotago.ToBeSignedUTXOInput
		var outputs []*iotago.SigLockedSingleOutput
		tempValue = needValue
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
				inputUTXO := &iotago.ToBeSignedUTXOInput{
					Address: &edAddr,
					Input:   &iotago.UTXOInput{},
				}
				if rawType.Amount > tempValue {
					returnValue = rawType.Amount - tempValue
					tempValue = 0
				} else {
					tempValue = tempValue - rawType.Amount
				}
				transactionID, _ := hex.DecodeString(outputRes.TransactionID)
				copy(inputUTXO.Input.TransactionID[:], transactionID)
				inputUTXO.Input.TransactionOutputIndex = outputRes.OutputIndex
				inputs = append(inputs, inputUTXO)
				if returnValue > 0 || tempValue == 0 {
					break
				}
			}
		}

		outputs = append(outputs, &iotago.SigLockedSingleOutput{
			Address: toEdAddr, Amount: needValue,
		})
		if returnValue > 0 {
			outputs = append(outputs, &iotago.SigLockedSingleOutput{
				Address: &edAddr, Amount: returnValue,
			})
		}
		messageBuilder := iota.BuildMessage(inputs, outputs, indexationPayload)

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
		}
		//else {
		// 	if signMessage, err := messageBuilder.Essence.SigningMessage(); err != nil {
		// 		log.Fatal("get signMessage error", "err", err)
		// 	} else {
		// 		var signature serializer.Serializable
		// 		unlockBlocks := serializer.Serializables{}
		// 		signature, err = signer.Sign(&edAddr, signMessage)
		// 		if err != nil {
		// 			log.Fatal("Sign error", "err", err)
		// 		} else {
		// 			for i := 0; i < len(inputs); i++ {
		// 				switch i {
		// 				case 0:
		// 					unlockBlocks = append(unlockBlocks, &iotago.SignatureUnlockBlock{Signature: signature})
		// 				default:
		// 					unlockBlocks = append(unlockBlocks, &iotago.ReferenceUnlockBlock{Reference: uint16(0)})
		// 				}
		// 			}
		// 		}
		// 		sigTxPayload := &iotago.Transaction{Essence: messageBuilder.Essence, UnlockBlocks: unlockBlocks}
		// 		if message, err := iotago.NewMessageBuilder().Payload(sigTxPayload).Build(); err != nil {
		// 			log.Fatal("NewTransactionBuilder", "err", err)
		// 		} else {
		// 			if res, err := nodeHTTPAPIClient.SubmitMessage(ctx, message); err != nil {
		// 				log.Fatal("SubmitMessage", "err", err)
		// 			} else {
		// 				fmt.Printf("res: %+v\n", iotago.MessageIDToHexString(res.MustID()))
		// 			}
		// 		}
		// 	}
		// }
	}
}

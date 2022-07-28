package iota

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	ctx = context.Background()
)

func GetTransactionMetadata(url string, msgID [32]byte) (*iotago.MessageMetadataResponse, error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if metadataResponse, err := nodeHTTPAPIClient.MessageMetadataByMessageID(ctx, msgID); err != nil {
		return nil, err
	} else {
		if *metadataResponse.LedgerInclusionState != "included" {
			return nil, tokens.ErrTxIsNotValidated
		} else {
			return metadataResponse, nil
		}
	}
}

func GetTransactionByHash(url string, msgID [32]byte) (*iotago.Message, error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if messageRes, err := nodeHTTPAPIClient.MessageByMessageID(ctx, msgID); err != nil {
		return nil, err
	} else {
		return messageRes, nil
	}
}

func GetLatestBlockNumber(url string) (uint64, error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if nodeInfoResponse, err := nodeHTTPAPIClient.Info(ctx); err != nil {
		return 0, err
	} else {
		return uint64(nodeInfoResponse.ConfirmedMilestoneIndex), nil
	}
}

func GetOutPutIDs(url string, edAddr *iotago.Ed25519Address) ([]iotago.OutputIDHex, error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if outputResponse, _, err := nodeHTTPAPIClient.OutputsByEd25519Address(ctx, edAddr, false); err != nil {
		return nil, err
	} else {
		return outputResponse.OutputIDs, nil
	}
}

func GetOutPutByID(url string, outputID iotago.UTXOInputID, needValue uint64) (*iotago.UTXOInput, bool, uint64, error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if outputRes, err := nodeHTTPAPIClient.OutputByID(ctx, outputID); err != nil {
		return nil, false, 0, err
	} else {
		var rawType *RawType
		rawOutPut, _ := outputRes.RawOutput.MarshalJSON()
		if err := json.Unmarshal(rawOutPut, &rawType); err != nil {
			return nil, false, 0, err
		} else {
			if transactionID, err := hex.DecodeString(outputRes.TransactionID); err != nil {
				return nil, false, 0, err
			} else {
				utxoInput := &iotago.UTXOInput{}
				var flag bool
				var amount uint64
				copy(utxoInput.TransactionID[:], transactionID)
				utxoInput.TransactionOutputIndex = outputRes.OutputIndex
				if rawType.Amount < needValue {
					amount = needValue - rawType.Amount
					flag = false
				} else {
					amount = rawType.Amount - needValue
					flag = true
				}
				return utxoInput, flag, amount, nil
			}
		}
	}
}

func CommitMessage(url string, message *iotago.Message) (string, error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if res, err := nodeHTTPAPIClient.SubmitMessage(ctx, message); err == nil {
		return iotago.MessageIDToHexString(res.MustID()), nil
	} else {
		return "", err
	}
}

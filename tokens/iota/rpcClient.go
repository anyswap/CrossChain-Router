package iota

import (
	"context"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	ctx = context.Background()
)

func GetTransactionMetadata(url string, msgID [32]byte) (txRes *iotago.MessageMetadataResponse, err error) {
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

func GetTransactionByHash(url string, msgID [32]byte) (txRes *iotago.Message, err error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if messageRes, err := nodeHTTPAPIClient.MessageByMessageID(ctx, msgID); err != nil {
		return nil, err
	} else {
		return messageRes, nil
	}
}

func GetLatestBlockNumber(url string) (num uint64, err error) {
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(url)
	if nodeInfoResponse, err := nodeHTTPAPIClient.Info(ctx); err != nil {
		return 0, err
	} else {
		return uint64(nodeInfoResponse.ConfirmedMilestoneIndex), nil
	}
}

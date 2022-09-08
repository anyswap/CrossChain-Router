package cosmos

import "github.com/cosmos/cosmos-sdk/types"

type CosmosRestClient struct {
	BaseUrls []string
}

type GetTxResponse struct {
	// tx_response is the queried TxResponses.
	TxResponse *types.TxResponse `protobuf:"bytes,2,opt,name=tx_response,json=txResponse,proto3" json:"tx_response,omitempty"`
}

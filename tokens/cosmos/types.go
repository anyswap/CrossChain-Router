package cosmos

import (
	v1beta11 "cosmossdk.io/api/cosmos/base/abci/v1beta1"
	"github.com/cosmos/cosmos-sdk/types"
)

type CosmosRestClient struct {
	BaseUrls []string
}

type GetTxResponse struct {
	// tx_response is the queried TxResponses.
	TxResponse *types.TxResponse `protobuf:"bytes,2,opt,name=tx_response,json=txResponse,proto3" json:"tx_response,omitempty"`
}

type BroadcastTxRequest struct {
	TxBytes string `json:"tx_bytes"`
	Mode    string `json:"mode"`
}

type BroadcastTxResponse struct {
	TxResponse *v1beta11.TxResponse `protobuf:"bytes,1,opt,name=tx_response,json=txResponse,proto3" json:"tx_response,omitempty"`
}

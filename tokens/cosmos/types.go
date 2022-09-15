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
	TxResponse *TxResponse `protobuf:"bytes,2,opt,name=tx_response,json=txResponse,proto3" json:"tx_response,omitempty"`
}

// TxResponse defines a structure containing relevant tx data and metadata. The
// tags are stringified and the log is JSON decoded.
type TxResponse struct {
	// The block height
	Height int64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	// The transaction hash.
	TxHash string `protobuf:"bytes,2,opt,name=txhash,proto3" json:"txhash,omitempty"`
	// Response code.
	Code uint32 `protobuf:"varint,4,opt,name=code,proto3" json:"code,omitempty"`
	// The output of the application's logger (typed). May be non-deterministic.
	Logs types.ABCIMessageLogs `protobuf:"bytes,7,rep,name=logs,proto3,castrepeated=ABCIMessageLogs" json:"logs"`
	// The request transaction bytes.
	Tx interface{} `protobuf:"bytes,11,opt,name=tx,proto3" json:"tx,omitempty"`
}

type BroadcastTxRequest struct {
	TxBytes string `json:"tx_bytes"`
	Mode    string `json:"mode"`
}

type BroadcastTxResponse struct {
	TxResponse *v1beta11.TxResponse `protobuf:"bytes,1,opt,name=tx_response,json=txResponse,proto3" json:"tx_response,omitempty"`
}

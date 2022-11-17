package cosmos

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
)

type BuildRawTx struct {
	TxBuilder *cosmosClient.TxBuilder `protobuf:"bytes,2,opt,name=tx_builder,proto3" json:"tx_builder,omitempty"`
	Extra     *tokens.AllExtras       `protobuf:"bytes,2,opt,name=extra,proto3" json:"extra,omitempty"`
}

// GetLatestBlockResponse is the response type for the Query/GetLatestBlock RPC
// method.
type GetLatestBlockResponse struct {
	// Deprecated: please use `sdk_block` instead
	Block *Block `protobuf:"bytes,2,opt,name=block,proto3" json:"block,omitempty"`
}

type Block struct {
	Header Header `protobuf:"bytes,1,opt,name=header,proto3" json:"header"`
}

// Header defines the structure of a Tendermint block header.
type Header struct {
	// basic block info
	ChainID string `protobuf:"bytes,2,opt,name=chain_id,json=chainId,proto3" json:"chain_id,omitempty"`
	Height  string `protobuf:"varint,3,opt,name=height,proto3" json:"height,omitempty"`
}

type GetTxResponse struct {
	Status string `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
	Msg    string `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
	// The request transaction bytes.
	Tx *Tx `protobuf:"bytes,11,opt,name=tx,proto3" json:"tx,omitempty"`
	// tx_response is the queried TxResponses.
	TxResponse *TxResponse `protobuf:"bytes,2,opt,name=tx_response,json=txResponse,proto3" json:"tx_response,omitempty"`
}

// TxResponse defines a structure containing relevant tx data and metadata. The
// tags are stringified and the log is JSON decoded.
type TxResponse struct {
	// The block height
	Height string `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	// The transaction hash.
	TxHash string `protobuf:"bytes,2,opt,name=txhash,proto3" json:"txhash,omitempty"`
	// Response code.
	Code uint32 `protobuf:"varint,4,opt,name=code,proto3" json:"code,omitempty"`
	// The output of the application's logger (raw string). May be
	// non-deterministic.
	RawLog string `protobuf:"bytes,6,opt,name=raw_log,json=rawLog,proto3" json:"raw_log,omitempty"`
	// The output of the application's logger (typed). May be non-deterministic.
	Logs types.ABCIMessageLogs `protobuf:"bytes,7,rep,name=logs,proto3,castrepeated=ABCIMessageLogs" json:"logs"`
}

// Tx tx
type Tx struct {
	Body TxBody `protobuf:"bytes,1,opt,name=body,proto3" json:"body,omitempty"`
}

type TxBody struct {
	// memo is any arbitrary note/comment to be added to the transaction.
	// WARNING: in clients, any publicly exposed text should not be called memo,
	// but should be called `note` instead (see https://github.com/cosmos/cosmos-sdk/issues/9122).
	Memo string `protobuf:"bytes,2,opt,name=memo,proto3" json:"memo,omitempty"`
}

// SimulateRequest is the request type for the Service.Simulate
// RPC method.
type SimulateRequest struct {
	TxBytes string `json:"tx_bytes"`
}

type BroadcastTxRequest struct {
	TxBytes string `json:"tx_bytes"`
	Mode    string `json:"mode"`
}

type BroadcastTxResponse struct {
	TxResponse *TxResponse `protobuf:"bytes,1,opt,name=tx_response,json=txResponse,proto3" json:"tx_response,omitempty"`
}

// SimulateResponse is the response type for the
// Service.SimulateRPC method.
type SimulateResponse struct {
	// gas_info is the information about gas used in the simulation.
	GasInfo *GasInfo `protobuf:"bytes,1,opt,name=gas_info,json=gasInfo,proto3" json:"gas_info,omitempty"`
}

// GasInfo defines tx execution gas context.
type GasInfo struct {
	// GasUsed is the amount of gas actually consumed.
	GasUsed string `protobuf:"varint,2,opt,name=gas_used,json=gasUsed,proto3" json:"gas_used,omitempty"`
}

// QueryAccountResponse is the response type for the Query/Account RPC method.
type QueryAccountResponse struct {
	// account defines the account of the corresponding address.
	Account *BaseAccount `protobuf:"bytes,1,opt,name=account,proto3" json:"account,omitempty"`
	Status  string       `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
	Msg     string       `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
}

// BaseAccount base account
type BaseAccount struct {
	Address       string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	AccountNumber string `protobuf:"varint,3,opt,name=account_number,json=accountNumber,proto3" json:"account_number,omitempty"`
	Sequence      string `protobuf:"varint,4,opt,name=sequence,proto3" json:"sequence,omitempty"`
}

type QueryAllBalancesResponse struct {
	// balances is the balances of all the coins.
	Balances types.Coins `protobuf:"bytes,1,rep,name=balances,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"balances"`
}

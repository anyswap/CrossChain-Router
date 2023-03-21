package reef

import "github.com/anyswap/CrossChain-Router/v3/common/hexutil"

const TxHash_GQL = "query query_tx_by_hash($hash: String!) {\n  extrinsic(where: {hash: {_eq: $hash}}) {\n    id\n    block_id\n    index\n    type\n    signer\n    section\n    method\n    args\n    hash\n    status\n    timestamp\n    error_message\n inherent_data\n signed_data\n  __typename\n  }\n}\n"
const EvmAddress_GQL = "subscription query_evm_addr($address: String!) {\n  account(\n where: {address: {_eq: $address}}) {\n  nonce\n evm_address\n  evm_address\n    __typename\n  }\n}\n"
const EventLog_GQL = "subscription query_eventlogs_by_extrinsic_id($extrinsic_id: bigint!) {\n  event(order_by: {index: asc}, where: {  _and: [ {extrinsic_id: {_eq: $extrinsic_id}}\n { method: { _eq: \"Log\" } }\n  ] }) {\n    extrinsic {\n      id\n      block_id\n      index\n      __typename\n    }\n    index\n    data\n    method\n    section\n    __typename\n  }\n}\n"
const ReefAddress_GQL = "subscription query_reef_addr($address: String!) {\n  account(\n where: {evm_address: {_eq: $address}}) {\n   nonce\n evm_address\n   address\n    __typename\n  }\n}\n"

type ReefGraphQLRequest struct {
	*Command
	// ID      string             `json:"id,omitempty"`
	Type    string             `json:"type"`
	Payload ReefGraphQLPayLoad `json:"payload"`
}

type ReefGraphQLPayLoad struct {
	Extensions    interface{}            `json:"extensions,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
	Query         string                 `json:"query,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

type ReefGraphQLBaseResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

type ReefGraphQLResponse struct {
	ReefGraphQLBaseResponse
	Payload ReefGraphQLResponsePayload `json:"payload,omitempty"`
}

type ReefGraphQLResponsePayload struct {
	Data interface{} `json:"data,omitempty"`
}

// @See https://github.com/reef-defi/reef-explorer/tree/develop/db/hasura/metadata/databases/reefexplorer/tables

type ReefGraphQLTxData struct {
	Extrinsic []Extrinsic `json:"extrinsic,omitempty"`
}

type BaseData struct {
	Index    uint64 `json:"index,omitempty"`
	Method   string `json:"method,omitempty"`
	Section  string `json:"section,omitempty"`
	Typename string `json:"__typename,omitempty"`
}

type Extrinsic struct {
	Args         []*hexutil.Bytes `json:"args,omitempty"`
	BlockID      *uint64          `json:"block_id,omitempty"`
	ID           *uint64          `json:"id,omitempty"`
	Signer       string           `json:"signer,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
	Hash         *string          `json:"hash,omitempty"`
	Timestamp    string           `json:"timestamp,omitempty"`
	Status       string           `json:"status,omitempty"`
	Type         string           `json:"type,omitempty"`
	SignedData   *SignedData      `json:"signed_data,omitempty"`
	BaseData
}

type SignedData struct {
	Fee *Fee `json:"fee,omitempty"`
}

type Fee struct {
	Class      string  `json:"class,omitempty"`
	PartialFee string  `json:"partialFee,omitempty"`
	Weight     *uint64 `json:"weight,omitempty"`
}

type ReefGraphQLEventLogsData struct {
	Events []EventLog `json:"event,omitempty"`
}
type EventLog struct {
	Data      []Log     `json:"data,omitempty"`
	Extrinsic Extrinsic `json:"extrinsic,omitempty"`
	BaseData
}

type Log struct {
	Data    *hexutil.Bytes `json:"data,omitempty"`
	Topics  []string       `json:"topics,omitempty"`
	Address string         `json:"address,omitempty"`
}

type ReefGraphQLAccountData struct {
	Accounts []Account `json:"account,omitempty"`
}

type Account struct {
	EvmAddress  string `json:"evm_address,omitempty"`
	ReefAddress string `json:"address,omitempty"`
	Nonce       uint64 `json:"nonce,omitempty"`
	EvmNonce    uint64 `json:"evm_nonce,omitempty"`
	BaseData
}

// Map message types to the appropriate data structure
var streamMessageFactory = map[string]func() interface{}{
	"init":     func() interface{} { return struct{}{} },
	"address":  func() interface{} { return &ReefGraphQLAccountData{} },
	"tx":       func() interface{} { return &ReefGraphQLTxData{} },
	"eventlog": func() interface{} { return &ReefGraphQLEventLogsData{} },
}

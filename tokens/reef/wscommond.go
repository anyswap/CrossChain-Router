package reef

import (
	"encoding/json"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

const TxHash_GQL = "query query_tx_by_hash($hash: String!) {\n  extrinsic(where: {hash: {_eq: $hash}}) {\n    id\n    block_id\n    index\n    type\n    signer\n    section\n    method\n    args\n    hash\n    status\n    timestamp\n    error_message\n inherent_data\n signed_data\n  __typename\n  }\n}\n"
const EvmAddress_GQL = "subscription query_evm_addr($address: String!) {\n  account(\n where: {address: {_eq: $address}}) {\n  nonce\n evm_address\n  evm_address\n    __typename\n  }\n}\n"
const EventLog_GQL = "subscription query_eventlogs_by_extrinsic_id($extrinsic_id: bigint!) {\n  event(order_by: {index: asc}, where: {  _and: [ {extrinsic_id: {_eq: $extrinsic_id}}\n { method: { _eq: \"Log\" } }\n  ] }) {\n    extrinsic {\n      id\n      block_id\n      index\n      __typename\n    }\n    index\n    data\n    method\n    section\n    __typename\n  }\n}\n"
const ReefAddress_GQL = "subscription query_reef_addr($address: String!) {\n  account(\n where: {evm_address: {_eq: $address}}) {\n   nonce\n evm_address\n   address\n    __typename\n  }\n}\n"

const TxHash_GQL_POST = "query extrinsics($hash: String!) {\n  extrinsics(where: {hash_eq: $hash}, limit: 1) {\n    id\n    block {\n   id\n   height\n      __typename\n    }\n    index\n    signer\n    section\n   status\n  method\n    args\n    hash\n    docs\n    type\n    timestamp\n    errorMessage\n    signedData\n    __typename\n  }\n}\n"
const EvmAddress_GQL_POST = "query accounts($address: String!) {\n  accounts(\n where: {id_eq: $address}) {\n  nonce\n evmAddress\n  id\n evmNonce\n   __typename\n  }\n}\n"
const ReefAddress_GQL_POST = "query accounts($address: String!){\n  accounts(\n where: {evmAddress_containsInsensitive: $address}, limit: 1) {\n   nonce\n evmAddress\n   id\n  evmNonce\n  __typename\n  }\n}\n"
const EventLog_GQL_POST = "query evmEvents($hash: String!) {\n evmEvents(where: {event: {extrinsic: {hash_eq: $hash}}}, limit: 90) {\n method\n id\n extrinsicIndex\n status\n eventIndex\n dataRaw\n contractAddress\n }\n}\n"

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
	BaseData
	Args         []interface{} `json:"args,omitempty"`
	ID           string        `json:"id,omitempty"`
	Signer       string        `json:"signer,omitempty"`
	ErrorMessage string        `json:"errorMessage,omitempty"`
	Hash         *string       `json:"hash,omitempty"`
	Timestamp    string        `json:"timestamp,omitempty"`
	Status       string        `json:"status,omitempty"`
	Type         string        `json:"type,omitempty"`
	SignedData   *SignedData   `json:"signedData,omitempty"`
	Block        *Block        `json:"block,omitempty"`
	Index        int           `json:"index,omitempty"`
	// BlockID      *uint64       `json:"block_id,omitempty"`
}

type Block struct {
	ID     string  `json:"id,omitempty"`
	Height *uint64 `json:"height,omitempty"`
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
	EvmAddress       string `json:"evmAddress,omitempty"`
	ReefAddress      string `json:"id,omitempty"`
	Nonce            uint64 `json:"nonce,omitempty"`
	EvmNonce         uint64 `json:"evmNonce,omitempty"`
	AvailableBalance uint64 `json:"availableBalance,omitempty"`
	BaseData
}

// Map message types to the appropriate data structure
var streamMessageFactory = map[string]func() interface{}{
	"init":     func() interface{} { return struct{}{} },
	"address":  func() interface{} { return &ReefGraphQLAccountData{} },
	"tx":       func() interface{} { return &ReefGraphQLTxData{} },
	"eventlog": func() interface{} { return &ReefGraphQLEventLogsData{} },
}

type ReefGraphQL interface {
	QueryTx(hash string) (*Extrinsic, error)
	QueryEventLogs(extrinsicId uint64) (*[]EventLog, error)
	QueryEvmAddress(ss58address string) (*common.Address, error)
	QueryAccountByReefAddr(ss58address string) (*Account, error)
	QueryReefAddress(evmAddress string) (*string, error)
}

type ReefGraphQLImpl struct {
	WS  *WebSocket
	Uri string
}

func reefGraphQLPost(url string, reqBody string, result interface{}) error {
	resp, err := client.RPCJsonPostWithTimeout(url, reqBody, 30)
	if err != nil {
		log.Trace("post rpc error", "url", url, "request", reqBody, "err", err)
		return err
	}
	err = json.Unmarshal([]byte(resp), result)
	if err != nil {
		log.Trace("json Unmarshal error", "url", url, "err", err)
		return err
	}
	return nil
}

type QueryTxResp struct {
	Data QueryTxRespData `json:"data,omitempty"`
}

type QueryTxRespData struct {
	Extrinsics []Extrinsic `json:"extrinsics,omitempty"`
}

func (gql ReefGraphQLImpl) QueryTx(hash string) (*Extrinsic, error) {
	if gql.WS != nil {
		return gql.WS.QueryTx(hash)
	}

	body := ReefGraphQLPayLoad{
		OperationName: "extrinsics",
		Query:         TxHash_GQL_POST,
		Variables: map[string]interface{}{
			"hash": hash,
		}}
	json, _ := json.Marshal(body)

	resq := &QueryTxResp{}
	err := reefGraphQLPost(gql.Uri, string(json), resq)
	if err != nil {
		return nil, err
	}
	if len(resq.Data.Extrinsics) > 0 {
		return &resq.Data.Extrinsics[0], nil
	}
	return nil, nil
}

type EventLogsResp struct {
	Data EventLogsData `json:"data,omitempty"`
}

type EventLogsData struct {
	EvmEvents []EvmEvents `json:"evmEvents,omitempty"`
}

type EvmEvents struct {
	BaseData
	ID         string `json:"id,omitempty"`
	EventIndex int    `json:"eventIndex,omitempty"`
	Log        Log    `json:"dataRaw,omitempty"`
}

func (gql ReefGraphQLImpl) QueryEventLogs(extrinsicHash string) (*[]EventLog, error) {
	body := ReefGraphQLPayLoad{
		OperationName: "evmEvents",
		Query:         EventLog_GQL_POST,
		Variables: map[string]interface{}{
			"hash": extrinsicHash,
		}}
	json, _ := json.Marshal(body)

	resq := &EventLogsResp{}
	err := reefGraphQLPost(gql.Uri, string(json), resq)
	if err != nil {
		return nil, err
	}
	log := EventLog{
		Data: []Log{},
		Extrinsic: Extrinsic{
			Hash: &extrinsicHash,
		},
	}
	for _, events := range resq.Data.EvmEvents {
		log.Data = append(log.Data, events.Log)
	}
	return &[]EventLog{log}, nil
}

type AccountsResp struct {
	Data AccountData `json:"data,omitempty"`
}

type AccountData struct {
	Accounts []Account `json:"accounts,omitempty"`
}

func (gql ReefGraphQLImpl) QueryAccountByReefAddr(ss58address string) (*Account, error) {
	if gql.WS != nil {
		return gql.WS.QueryAccountByReefAddr(ss58address)
	}
	body := ReefGraphQLPayLoad{
		OperationName: "accounts",
		Query:         EvmAddress_GQL_POST,
		Variables: map[string]interface{}{
			"address": ss58address,
		}}
	json, _ := json.Marshal(body)

	resq := &AccountsResp{}
	err := reefGraphQLPost(gql.Uri, string(json), resq)
	if err != nil {
		return nil, err
	}
	if len(resq.Data.Accounts) > 0 {
		return &resq.Data.Accounts[0], nil
	}
	return nil, nil
}

func (gql ReefGraphQLImpl) QueryEvmAddress(ss58address string) (*common.Address, error) {
	if gql.WS != nil {
		return gql.WS.QueryEvmAddress(ss58address)
	}

	account, err := gql.QueryAccountByReefAddr(ss58address)
	if err != nil || account == nil {
		return nil, err
	}
	evmAddr := common.HexToAddress(account.EvmAddress)
	return &evmAddr, nil
}

func (gql ReefGraphQLImpl) QueryReefAddress(evmAddress string) (*string, error) {
	if gql.WS != nil {
		return gql.WS.QueryReefAddress(evmAddress)
	}
	body := ReefGraphQLPayLoad{
		OperationName: "accounts",
		Query:         ReefAddress_GQL_POST,
		Variables: map[string]interface{}{
			"address": evmAddress,
		}}
	json, _ := json.Marshal(body)

	resq := &AccountsResp{}
	err := reefGraphQLPost(gql.Uri, string(json), resq)
	if err != nil {
		return nil, err
	}
	if len(resq.Data.Accounts) > 0 {
		return &resq.Data.Accounts[0].ReefAddress, nil
	}
	return nil, nil
}

package reef

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
	Args         []string `json:"args,omitempty"`
	BlockID      uint64   `json:"block_id,omitempty"`
	ID           uint64   `json:"id,omitempty"`
	Signer       string   `json:"signer,omitempty"`
	ErrorMessage string   `json:"error_message,omitempty"`
	Hash         string   `json:"hash,omitempty"`
	Timestamp    string   `json:"timestamp,omitempty"`
	Status       string   `json:"status,omitempty"`
	Type         string   `json:"type,omitempty"`
	BaseData
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
	Data    string   `json:"data,omitempty"`
	Topics  []string `json:"topics,omitempty"`
	Address string   `json:"address,omitempty"`
}

// Map message types to the appropriate data structure
var streamMessageFactory = map[string]func() interface{}{
	"init":     func() interface{} { return struct{}{} },
	"tx":       func() interface{} { return &ReefGraphQLTxData{} },
	"eventlog": func() interface{} { return &ReefGraphQLEventLogsData{} },
}

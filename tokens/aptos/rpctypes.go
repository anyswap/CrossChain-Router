package aptos

type LedgerInfo struct {
	ChainId             uint   `json:"chain_id"`
	Epoch               string `json:"epoch"`
	LedgerVersion       string `json:"ledger_version"`
	OldestLedgerVersion string `json:"oldest_ledger_version"`
	BlockHeight         string `json:"block_height"`
	OldestBlockHeight   string `json:"oldest_block_height"`
	LedgerTimestamp     string `json:"ledger_timestamp"`
	NodeRole            string `json:"node_role"`
}

type AccountInfo struct {
	SequenceNumber    string `json:"sequence_number"`
	AuthenticationKey string `json:"authentication_key"`
}

type CoinInfoResource struct {
	Type string           `json:"type"`
	Data CoinCoinInfoData `json:"data"`
}

type CoinCoinInfoData struct {
	Decimals uint8  `json:"decimals"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
}

type CoinStoreResource struct {
	Type string         `json:"type"`
	Data *CoinStoreData `json:"data,omitempty"`
}
type CoinStoreData struct {
	Coin           CoinValue              `json:"coin"`
	DepositEvents  map[string]interface{} `json:"deposit_events,omitempty"`
	WithdrawEvents map[string]interface{} `json:"withdraw_events,omitempty"`
}

type CoinValue struct {
	Value string `json:"value"`
}

type BlockInfo struct {
	Height       string `json:"block_height"`
	Hash         string `json:"block_hash"`
	Timestamp    string `json:"block_timestamp"`
	FirstVersion string `json:"first_version"`
	LastVersion  string `json:"last_version"`
}

type TransactionInfo struct {
	Type                    string             `json:"type"`
	Version                 string             `json:"version"`
	Hash                    string             `json:"hash"`
	StateRootHash           string             `json:"state_root_hash,omitempty"`
	EventRootHash           string             `json:"event_root_hash,omitempty"`
	GasUsed                 string             `json:"gas_used,omitempty"`
	Success                 bool               `json:"success,omitempty"`
	VmStatus                string             `json:"vm_status,omitempty"`
	AccumulatorRootHash     string             `json:"accumulator_root_hash,omitempty"`
	Sender                  string             `json:"sender"`
	SequenceNumber          string             `json:"sequence_number"`
	MaxGasAmount            string             `json:"max_gas_amount"`
	GasUnitPrice            string             `json:"gas_unit_price"`
	ExpirationTimestampSecs string             `json:"expiration_timestamp_secs"`
	Timestamp               string             `json:"timestamp,omitempty"`
	PayLoad                 TransactionPayload `json:"payload,omitempty"`
	Events                  []Event            `json:"events,omitempty"`
}

type Event struct {
	Key            string            `json:"key,omitempty"`
	SequenceNumber string            `json:"sequence_number,omitempty"`
	Type           string            `json:"type,omitempty"`
	Data           map[string]string `json:"data,omitempty"` // TODO map[string]interface{}
}

type Transaction struct {
	Sender                  string                `json:"sender"`
	SequenceNumber          string                `json:"sequence_number"`
	MaxGasAmount            string                `json:"max_gas_amount"`
	GasUnitPrice            string                `json:"gas_unit_price"`
	GasCurrencyCode         string                `json:"gas_currency_code,omitempty"`
	ExpirationTimestampSecs string                `json:"expiration_timestamp_secs"`
	Payload                 *TransactionPayload   `json:"payload"`
	Signature               *TransactionSignature `json:"signature,omitempty"`
}
type TransactionPayload struct {
	Type          string        `json:"type"`
	Function      string        `json:"function"`
	TypeArguments []string      `json:"type_arguments"`
	Arguments     []interface{} `json:"arguments"`
}
type TransactionSignature struct {
	Type      string `json:"type,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type ScriptPayload struct {
	Type          string            `json:"type"`
	Code          ScriptPayloadCode `json:"code"`
	TypeArguments []string          `json:"type_arguments"`
	Arguments     []interface{}     `json:"arguments"`
}

type ScriptPayloadCode struct {
	Bytecode string `json:"bytecode"`
}

type ScriptTransaction struct {
	Sender                  string                `json:"sender"`
	SequenceNumber          string                `json:"sequence_number"`
	MaxGasAmount            string                `json:"max_gas_amount"`
	GasUnitPrice            string                `json:"gas_unit_price"`
	GasCurrencyCode         string                `json:"gas_currency_code,omitempty"`
	ExpirationTimestampSecs string                `json:"expiration_timestamp_secs"`
	Payload                 *ScriptPayload        `json:"payload"`
	Signature               *TransactionSignature `json:"signature,omitempty"`
}

// type ModuleTransaction struct {
// 	Sender                  string                `json:"sender"`
// 	SequenceNumber          string                `json:"sequence_number"`
// 	MaxGasAmount            string                `json:"max_gas_amount"`
// 	GasUnitPrice            string                `json:"gas_unit_price"`
// 	GasCurrencyCode         string                `json:"gas_currency_code,omitempty"`
// 	ExpirationTimestampSecs string                `json:"expiration_timestamp_secs"`
// 	Payload                 *ModulePayload        `json:"payload"`
// 	Signature               *TransactionSignature `json:"signature,omitempty"`
// }

// type ModulePayload struct {
// 	Type    string          `json:"type"`
// 	Modules *[]ModuleDefine `json:"modules"`
// }

// type ModuleDefine struct {
// 	Bytecode string `json:"bytecode"`
// }

type EventData struct {
	Version         string `json:"version"`
	Key             string `json:"key"`
	Sequence_number string `json:"sequence_number"`
	Type            string `json:"type"`
}

type CoinEvent struct {
	EventData
	Data CoinEventData `json:"data"`
}

type CoinEventData struct {
	Amount string `json:"amount"`
}

type SwapinEvent struct {
	EventData
	Data SwapinData `json:"data"`
}

type SwapinData struct {
	SwapID      string `json:"swapid"`
	Token       string `json:"token"`
	To          string `json:"to"`
	Amount      string `json:"amount"`
	FromChainId string `json:"from_chain_id"`
}

type SwapoutEvent struct {
	EventData
	Data SwapoutData `json:"data"`
}

type SwapoutData struct {
	Token     string `json:"token"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    string `json:"amount"`
	ToChainId string `json:"to_chain_id"`
}

type GasEstimate struct {
	GasPrice int `json:"gas_estimate"`
}

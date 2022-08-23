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
	Decimals string `json:"decimals"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
}

type CoinStoreResource struct {
	Type string        `json:"type"`
	Data CoinStoreData `json:"data"`
}
type CoinStoreData struct {
	Coin           CoinValue              `json:"coin"`
	DepositEvents  map[string]interface{} `json:"deposit_events,omitempty"`
	WithdrawEvents map[string]interface{} `json:"withdraw_events,omitempty"`
}

type CoinValue struct {
	Value string `json:"value"`
}

type TransactionInfo struct {
	Type                    string             `json:"type"`
	Version                 string             `json:"version,omitempty"`
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
	Data           map[string]string `json:"data,omitempty"`
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
	Type          string   `json:"type"`
	Function      string   `json:"function"`
	TypeArguments []string `json:"type_arguments"`
	Arguments     []string `json:"arguments"`
}
type TransactionSignature struct {
	Type      string `json:"type,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
}

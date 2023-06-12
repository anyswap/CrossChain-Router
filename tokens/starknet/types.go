package starknet

import (
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	HashLength = 32

	TxV0 = "0x0"
	TxV1 = "0x1"

	TxTypeInvoke = "INVOKE"
	TxTypeCall   = "CALL"

	NotReceived  = "NOT_RECEIVED"
	Received     = "RECEIVED"
	Pending      = "PENDING"
	AcceptedOnL1 = "ACCEPTED_ON_L1"
	AcceptedOnL2 = "ACCEPTED_ON_L2"
	Rejected     = "REJECTED"

	EC256K1    = "EC256K1"
	EC256STARK = "EC256STARK"
)

type Hash [HashLength]byte
type Signature []*big.Int

func (h Hash) Hex() string { return hexutil.Encode(h[:]) }

type ArrayFlag []string

type TransactionHash struct {
	TransactionHash Hash `json:"transaction_hash"`
}

func (i *ArrayFlag) String() string {
	return "a list of string params"
}

func (i *ArrayFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type Transaction interface {
	Hash() Hash
}

type Transactions []Transaction
type UnknownTransaction struct{ Transaction }

type CommonTransaction struct {
	TransactionHash Hash     `json:"transaction_hash,omitempty"`
	Type            string   `json:"type,omitempty"`
	MaxFee          string   `json:"max_fee,omitempty"`
	Version         string   `json:"version"`
	Signature       []string `json:"signature,omitempty"`
	Nonce           string   `json:"nonce,omitempty"`
}

type InvokeTxnV0 struct {
	CommonTransaction
	ContractAddress    Hash     `json:"contract_address"`
	EntryPointSelector string   `json:"entry_point_selector"`
	Calldata           []string `json:"calldata"`
}

type InvokeTxnV1 struct {
	CommonTransaction
	SenderAddress Hash     `json:"sender_address"`
	Calldata      []string `json:"calldata"`
}

type L1HandlerTxn struct {
	TransactionHash    Hash     `json:"transaction_hash,omitempty"`
	Type               string   `json:"type,omitempty"`
	Version            string   `json:"version"`
	Nonce              string   `json:"nonce,omitempty"`
	ContractAddress    Hash     `json:"contract_address"`
	EntryPointSelector string   `json:"entry_point_selector"`
	Calldata           []string `json:"calldata"`
}

type DeclareTxn struct {
	CommonTransaction
	ClassHash     string `json:"class_hash"`
	SenderAddress string `json:"sender_address"`
}

type DeployTxn struct {
	TransactionHash     Hash     `json:"transaction_hash,omitempty"`
	ClassHash           string   `json:"class_hash"`
	Type                string   `json:"type,omitempty"`
	Version             string   `json:"version"`
	ContractAddressSalt string   `json:"contract_address_salt"`
	ConstructorCalldata []string `json:"constructor_calldata"`
}

type DeployAccountTxn struct {
	CommonTransaction
	ClassHash           string   `json:"class_hash"`
	ContractAddressSalt string   `json:"contract_address_salt"`
	ConstructorCalldata []string `json:"constructor_calldata"`
}

type FunctionCall struct {
	ContractAddress    Hash     `json:"contract_address"`
	EntryPointSelector string   `json:"entry_point_selector,omitempty"`
	Calldata           []string `json:"calldata"`
}

type FunctionCallWithDetails struct {
	Call   FunctionCall
	MaxFee *big.Int
	Nonce  *big.Int
}

// InvokeV1 version 1 invoke transaction
type InvokeV1 struct {
	SenderAddress Hash     `json:"sender_address"`
	Calldata      []string `json:"calldata"`
}

type CommonTxnReceipt struct {
	TxHash      Hash   `json:"transaction_hash"`
	ActualFee   string `json:"actual_fee"`
	Status      string `json:"status"`
	BlockHash   Hash   `json:"block_hash"`
	BlockNumber uint64 `json:"block_number"`
	Type        string `json:"type,omitempty"`
}

type InvokeTxnReceiptProp struct {
	MessageSent []MsgToL1 `json:"messages_sent,omitempty"`
	Events      []Event   `json:"events,omitempty"`
}

type MsgToL1 struct {
	ToAddress string   `json:"to_address"`
	Payload   []string `json:"payload"`
}

type Event struct {
	FromAddress Hash     `json:"from_address"`
	Keys        []string `json:"keys"`
	Data        []string `json:"data"`
}

type InvokeTransactionReceipt struct {
	CommonTxnReceipt
	InvokeTxnReceiptProp `json:",omitempty"`
}

type SwapIn struct {
	Tx          string `json:"tx"`
	Token       string `json:"token"`
	To          string `json:"to"`
	FromChainId string `json:"fromChainId"`
	Amount      uint64 `json:"amount"`
}

func (s *SwapIn) getCalldata() []string {
	var calldata []string
	calldata = append(calldata, s.Tx)
	calldata = append(calldata, s.Token)
	calldata = append(calldata, s.To)
	calldata = append(calldata, strconv.FormatUint(s.Amount, 10))
	calldata = append(calldata, s.FromChainId)
	return calldata
}

type ExecuteDetails struct {
	MaxFee *big.Int
	Nonce  *big.Int
}

type FunctionInvoke struct {
	MaxFee *big.Int `json:"max_fee"`
	// Version of the transaction scheme, should be set to 0 or 1
	Version *big.Int `json:"version"`
	// Signature
	Signature Signature `json:"signature"`
	// Nonce should only be set with Transaction V1
	Nonce *big.Int `json:"nonce,omitempty"`
	// Defines the transaction type to invoke
	Type string `json:"type,omitempty"`

	FunctionCall
}

type FeeEstimate struct {
	GasConsumed string `json:"gas_consumed"`
	GasPrice    string `json:"gas_price"`
	OverallFee  string `json:"overall_fee"`
}

type AddInvokeTransactionOutput struct {
	TransactionHash string `json:"transaction_hash"`
}

type BlockID struct {
	Number *uint64 `json:"block_number,omitempty"`
	Hash   *Hash   `json:"block_hash,omitempty"`
	Tag    string  `json:"block_tag,omitempty"`
}

func (tx *InvokeTxnV0) Hash() Hash {
	return tx.TransactionHash
}

func (tx *InvokeTxnV1) Hash() Hash {
	return tx.TransactionHash
}

func (tx *DeclareTxn) Hash() Hash {
	return tx.TransactionHash
}

func (tx *DeployTxn) Hash() Hash {
	return tx.TransactionHash
}

func (tx *DeployAccountTxn) Hash() Hash {
	return tx.TransactionHash
}

func (tx *L1HandlerTxn) Hash() Hash {
	return tx.TransactionHash
}

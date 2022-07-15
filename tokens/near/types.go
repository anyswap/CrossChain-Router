package near

import (
	"math/big"

	"github.com/near/borsh-go"
)

type TransactionResult struct {
	Status             Status             `json:"status"`
	Transaction        Transaction        `json:"transaction"`
	TransactionOutcome TransactionOutcome `json:"transaction_outcome"`
	ReceiptsOutcome    []ReceiptsOutcome  `json:"receipts_outcome"`
}

type BlockDetail struct {
	Header BlockHeader `json:"header"`
}

type BlockHeader struct {
	Hash   string `json:"hash"`
	Height uint64 `json:"height"`
}

type Status struct {
	SuccessValue     interface{} `json:"SuccessValue,omitempty"`
	SuccessReceiptID interface{} `json:"SuccessReceiptId,omitempty"`
	Failure          interface{} `json:"Failure,omitempty"`
	Unknown          interface{} `json:"Unknown,omitempty"`
}

type Transaction struct {
	// Actions    []Action `json:"actions"`
	Hash       string `json:"hash"`
	Nonce      uint64 `json:"nonce"`
	PublicKey  string `json:"public_key"`
	ReceiverID string `json:"receiver_id"`
	Signature  string `json:"signature"`
	SignerID   string `json:"signer_id"`
}

type TransactionOutcome struct {
	BlockHash string  `json:"block_hash"`
	ID        string  `json:"id"`
	Outcome   Outcome `json:"outcome"`
	Proof     []Proof `json:"proof"`
}

type ReceiptsOutcome struct {
	BlockHash string  `json:"block_hash"`
	ID        string  `json:"id"`
	Outcome   Outcome `json:"outcome"`
	Proof     []Proof `json:"proof"`
}

type Outcome struct {
	ExecutorID  string   `json:"executor_id"`
	GasBurnt    int64    `json:"gas_burnt"`
	Logs        []string `json:"logs"`
	ReceiptIds  []string `json:"receipt_ids"`
	Status      Status   `json:"status"`
	TokensBurnt string   `json:"tokens_burnt"`
}

type Proof struct {
	Direction string `json:"direction"`
	Hash      string `json:"hash"`
}

type Action struct {
	Enum           borsh.Enum `borsh_enum:"true"` // treat struct as complex enum when serializing/deserializing
	CreateAccount  borsh.Enum
	DeployContract DeployContract
	FunctionCall   FunctionCall
	Transfer       Transfer
	Stake          Stake
	AddKey         AddKey
	DeleteKey      DeleteKey
	DeleteAccount  DeleteAccount
}

// The DeployContract action.
type DeployContract struct {
	Code []byte
}

// The FunctionCall action.
type FunctionCall struct {
	MethodName string
	Args       []byte
	Gas        uint64
	Deposit    big.Int // u128
}

// The Stake action.
type Stake struct {
	Stake     big.Int // u128
	PublicKey PublicKey
}

// The AddKey action.
type AddKey struct {
	PublicKey PublicKey
	AccessKey AccessKey
}

// AccessKey encodes a NEAR access key.
type AccessKey struct {
	Nonce      uint64
	Permission AccessKeyPermission
}

// AccessKeyPermission encodes a NEAR access key permission.
type AccessKeyPermission struct {
	Enum         borsh.Enum `borsh_enum:"true"` // treat struct as complex enum when serializing/deserializing
	FunctionCall FunctionCallPermission
	FullAccess   borsh.Enum
}

// FunctionCallPermission encodes a NEAR function call permission (an access
// key permission).
type FunctionCallPermission struct {
	Allowance   *big.Int
	ReceiverID  string
	MethodNames []string
}

// The DeleteAccount action.
type DeleteAccount struct {
	BeneficiaryID string
}

// The DeleteKey action.
type DeleteKey struct {
	PublicKey PublicKey
}

type Transfer struct {
	Deposit big.Int
}

// A Transaction encodes a NEAR transaction.
type RawTransaction struct {
	SignerID   string
	PublicKey  PublicKey
	Nonce      uint64
	ReceiverID string
	BlockHash  [32]byte
	Actions    []Action
}

// PublicKey encoding for NEAR.
type PublicKey struct {
	KeyType uint8
	Data    [32]byte
}

type SignedTransaction struct {
	Transaction RawTransaction
	Signature   Signature
}

// A Signature used for signing transaction.
type Signature struct {
	KeyType uint8
	Data    [64]byte
}

type FtTransfer struct {
	ReceiverId string `json:"receiver_id"`
	Amount     string `json:"amount"`
	Memo       string `json:"memo"`
}

type FunctionCallResult struct {
	BlockHash   string   `json:"block_hash"`
	BlockHeight uint64   `json:"block_height"`
	Logs        []string `json:"logs"`
	Result      []byte   `json:"result"`
	Error       string   `json:"error,omitempty"`
}

type FungibleTokenMetadata struct {
	Spec          string `json:"spec"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	Icon          string `json:"icon"`
	Reference     string `json:"reference"`
	ReferenceHash string `json:"reference_hash"`
	Decimals      uint8  `json:"decimals"`
}

type CreateAccount struct {
	NewAccountId string `json:"new_account_id"`
	NewPublicKey string `json:"new_public_key"`
}

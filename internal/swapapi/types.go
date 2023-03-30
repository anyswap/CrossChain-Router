package swapapi

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
)

// MapIntResult type
type MapIntResult map[int]string

// ServerInfo serverinfo
type ServerInfo struct {
	Identifier     string
	Version        string
	ExtraConfig    *params.ExtraConfig `json:",omitempty"`
	AllChainIDs    []*big.Int
	PausedChainIDs []*big.Int `json:",omitempty"`
}

// OracleInfo oracle info
type OracleInfo struct {
	Heartbeat          string
	HeartbeatTimestamp int64
}

// SwapInfo swap info
type SwapInfo struct {
	SwapType      uint32             `json:"swaptype"`
	TxID          string             `json:"txid"`
	TxTo          string             `json:"txto,omitempty"`
	TxHeight      uint64             `json:"txheight"`
	From          string             `json:"from"`
	To            string             `json:"to"`
	Bind          string             `json:"bind"`
	Value         string             `json:"value"`
	LogIndex      int                `json:"logIndex,omitempty"`
	FromChainID   string             `json:"fromChainID"`
	ToChainID     string             `json:"toChainID"`
	SwapInfo      mongodb.SwapInfo   `json:"swapinfo"`
	SwapTx        string             `json:"swaptx"`
	SwapHeight    uint64             `json:"swapheight"`
	SwapValue     string             `json:"swapvalue"`
	SwapNonce     uint64             `json:"swapnonce"`
	Status        mongodb.SwapStatus `json:"status"`
	StatusMsg     string             `json:"statusmsg"`
	InitTime      int64              `json:"inittime"`
	Timestamp     int64              `json:"timestamp"`
	Memo          string             `json:"memo,omitempty"`
	ReplaceCount  int                `json:"replaceCount,omitempty"`
	Confirmations uint64             `json:"confirmations"`
	ProofID       string             `json:"proofID,omitempty"`
	Proof         string             `json:"proof,omitempty"`
}

// ChainConfig rpc type
type ChainConfig struct {
	ChainID        string
	BlockChain     string
	RouterContract string
	RouterVersion  string `json:",omitempty"`
	Confirmations  uint64
	InitialHeight  uint64
}

// TokenConfig rpc type
type TokenConfig struct {
	TokenID         string
	Decimals        uint8
	ContractAddress string
	ContractVersion uint64
	RouterContract  string
	RouterVersion   string `json:",omitempty"`
	Underlying      string
}

// SwapConfig rpc type
type SwapConfig struct {
	MaximumSwap       string
	MinimumSwap       string
	BigValueThreshold string
}

// FeeConfig rpc type
type FeeConfig struct {
	SwapFeeRatePerMillion uint64
	MaximumSwapFee        string
	MinimumSwapFee        string
}

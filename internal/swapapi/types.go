package swapapi

import (
	"github.com/anyswap/CrossChain-Router/mongodb"
)

// MapIntResult type
type MapIntResult map[int]string

// ServerInfo serverinfo
type ServerInfo struct {
	Identifier     string
	Version        string
	ConfigContract string
}

// SwapInfo swap info
type SwapInfo struct {
	SwapType      uint32             `json:"swaptype"`
	TxID          string             `json:"txid"`
	TxTo          string             `json:"txto"`
	TxHeight      uint64             `json:"txheight"`
	TxTime        uint64             `json:"txtime"`
	From          string             `json:"from"`
	To            string             `json:"to"`
	Bind          string             `json:"bind"`
	Value         string             `json:"value"`
	LogIndex      int                `json:"logIndex,omitempty"`
	FromChainID   string             `json:"fromChainID"`
	ToChainID     string             `json:"toChainID"`
	SwapInfo      mongodb.SwapInfo   `json:"swapinfo"`
	SwapTx        string             `json:"swaptx"`
	OldSwapTxs    []string           `json:"oldswaptxs,omitempty"`
	SwapHeight    uint64             `json:"swapheight"`
	SwapTime      uint64             `json:"swaptime"`
	SwapValue     string             `json:"swapvalue"`
	SwapNonce     uint64             `json:"swapnonce"`
	Status        mongodb.SwapStatus `json:"status"`
	StatusMsg     string             `json:"statusmsg"`
	InitTime      int64              `json:"inittime"`
	Timestamp     int64              `json:"timestamp"`
	Memo          string             `json:"memo,omitempty"`
	Confirmations uint64             `json:"confirmations"`
}

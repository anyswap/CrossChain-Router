package swapapi

import (
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
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
	ReplaceCount  int                `json:"replaceCount"`
	Confirmations uint64             `json:"confirmations"`
}

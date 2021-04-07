package swapapi

import (
	"github.com/anyswap/CrossChain-Router/mongodb"
)

// MapIntResult type
type MapIntResult map[int]string

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
	SwapInfo      mongodb.SwapInfo   `json:"swapinfo"`
	SwapTx        string             `json:"swaptx"`
	OldSwapTxs    []string           `json:"oldswaptxs"`
	SwapHeight    uint64             `json:"swapheight"`
	SwapTime      uint64             `json:"swaptime"`
	SwapValue     string             `json:"swapvalue"`
	SwapNonce     uint64             `json:"swapnonce"`
	Status        mongodb.SwapStatus `json:"status"`
	StatusMsg     string             `json:"statusmsg"`
	Timestamp     int64              `json:"timestamp"`
	Memo          string             `json:"memo"`
	Confirmations uint64             `json:"confirmations"`
}

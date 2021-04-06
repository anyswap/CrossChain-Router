package swapapi

import (
	"github.com/anyswap/CrossChain-Router/mongodb"
)

// SwapStatus type alias
type SwapStatus = mongodb.SwapStatus

// MapIntResult type
type MapIntResult map[int]string

// SwapInfo swap info
type SwapInfo struct {
	TxID          string     `json:"txid"`
	TxTo          string     `json:"txto"`
	TxHeight      uint64     `json:"txheight"`
	TxTime        uint64     `json:"txtime"`
	From          string     `json:"from"`
	To            string     `json:"to"`
	Bind          string     `json:"bind"`
	Value         string     `json:"value"`
	ForNative     bool       `json:"forNative,omitempty"`
	ForUnderlying bool       `json:"forUnderlying,omitempty"`
	Token         string     `json:"token,omitempty"`
	TokenID       string     `json:"tokenID,omitempty"`
	Path          []string   `json:"path,omitempty"`
	AmountOutMin  string     `json:"amountOutMin,omitempty"`
	FromChainID   string     `json:"fromChainID"`
	ToChainID     string     `json:"toChainID"`
	LogIndex      int        `json:"logIndex,omitempty"`
	SwapTx        string     `json:"swaptx"`
	SwapHeight    uint64     `json:"swapheight"`
	SwapTime      uint64     `json:"swaptime"`
	SwapValue     string     `json:"swapvalue"`
	SwapType      uint32     `json:"swaptype"`
	SwapNonce     uint64     `json:"swapnonce"`
	Status        SwapStatus `json:"status"`
	StatusMsg     string     `json:"statusmsg"`
	Timestamp     int64      `json:"timestamp"`
	Memo          string     `json:"memo"`
	Confirmations uint64     `json:"confirmations"`
}

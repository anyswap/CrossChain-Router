package mongodb

const (
	tbRouterSwaps       string = "RouterSwaps"
	tbRouterSwapResults string = "RouterSwapResults"
)

// MgoSwap registered swap
type MgoSwap struct {
	Key           string     `bson:"_id"` // fromChainID + txid + logindex
	SwapType      uint32     `bson:"swaptype"`
	TxID          string     `bson:"txid"`
	TxTo          string     `bson:"txto"`
	Bind          string     `bson:"bind"`
	ForNative     bool       `bson:"forNative,omitempty"`
	ForUnderlying bool       `bson:"forUnderlying,omitempty"`
	Token         string     `bson:"token"`
	TokenID       string     `bson:"tokenID"`
	Path          []string   `bson:"path,omitempty"`
	AmountOutMin  string     `bson:"amountOutMin,omitempty"`
	FromChainID   string     `bson:"fromChainID"`
	ToChainID     string     `bson:"toChainID"`
	LogIndex      int        `bson:"logIndex"`
	Status        SwapStatus `bson:"status"`
	Timestamp     int64      `bson:"timestamp"`
	Memo          string     `bson:"memo"`
}

// MgoSwapResult swap result (verified swap)
type MgoSwapResult struct {
	Key           string     `bson:"_id"` // fromChainID + txid + logindex
	SwapType      uint32     `bson:"swaptype"`
	TxID          string     `bson:"txid"`
	TxTo          string     `bson:"txto"`
	TxHeight      uint64     `bson:"txheight"`
	TxTime        uint64     `bson:"txtime"`
	From          string     `bson:"from"`
	To            string     `bson:"to"`
	Bind          string     `bson:"bind"`
	ForNative     bool       `bson:"forNative,omitempty"`
	ForUnderlying bool       `bson:"forUnderlying,omitempty"`
	Token         string     `bson:"token"`
	TokenID       string     `bson:"tokenID"`
	Path          []string   `bson:"path,omitempty"`
	AmountOutMin  string     `bson:"amountOutMin,omitempty"`
	FromChainID   string     `bson:"fromChainID"`
	ToChainID     string     `bson:"toChainID"`
	LogIndex      int        `bson:"logIndex"`
	Value         string     `bson:"value"`
	SwapTx        string     `bson:"swaptx"`
	OldSwapTxs    []string   `bson:"oldswaptxs"`
	SwapHeight    uint64     `bson:"swapheight"`
	SwapTime      uint64     `bson:"swaptime"`
	SwapValue     string     `bson:"swapvalue"`
	SwapNonce     uint64     `bson:"swapnonce"`
	Status        SwapStatus `bson:"status"`
	Timestamp     int64      `bson:"timestamp"`
	Memo          string     `bson:"memo"`
}

// SwapResultUpdateItems swap update items
type SwapResultUpdateItems struct {
	SwapTx     string
	OldSwapTxs []string
	SwapHeight uint64
	SwapTime   uint64
	SwapValue  string
	SwapNonce  uint64
	Status     SwapStatus
	Timestamp  int64
	Memo       string
}

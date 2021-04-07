package mongodb

const (
	tbRouterSwaps       string = "RouterSwaps"
	tbRouterSwapResults string = "RouterSwapResults"
)

// MgoSwap registered swap
type MgoSwap struct {
	Key       string `bson:"_id"` // fromChainID + txid + logindex
	SwapType  uint32 `bson:"swaptype"`
	TxID      string `bson:"txid"`
	TxTo      string `bson:"txto"`
	Bind      string `bson:"bind"`
	LogIndex  int    `bson:"logIndex"`
	SwapInfo  `bson:"swapinfo"`
	Status    SwapStatus `bson:"status"`
	Timestamp int64      `bson:"timestamp"`
	Memo      string     `bson:"memo"`
}

// MgoSwapResult swap result (verified swap)
type MgoSwapResult struct {
	Key        string `bson:"_id"` // fromChainID + txid + logindex
	SwapType   uint32 `bson:"swaptype"`
	TxID       string `bson:"txid"`
	TxTo       string `bson:"txto"`
	TxHeight   uint64 `bson:"txheight"`
	TxTime     uint64 `bson:"txtime"`
	From       string `bson:"from"`
	To         string `bson:"to"`
	Bind       string `bson:"bind"`
	Value      string `bson:"value"`
	LogIndex   int    `bson:"logIndex"`
	SwapInfo   `bson:"swapinfo"`
	SwapTx     string     `bson:"swaptx"`
	OldSwapTxs []string   `bson:"oldswaptxs"`
	SwapHeight uint64     `bson:"swapheight"`
	SwapTime   uint64     `bson:"swaptime"`
	SwapValue  string     `bson:"swapvalue"`
	SwapNonce  uint64     `bson:"swapnonce"`
	Status     SwapStatus `bson:"status"`
	Timestamp  int64      `bson:"timestamp"`
	Memo       string     `bson:"memo"`
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

// SwapInfo struct
type SwapInfo struct {
	*RouterSwapInfo  `bson:"routerSwapInfo,omitempty"`
	*AnyCallSwapInfo `bson:"anycallSwapInfo,omitempty"`
}

// RouterSwapInfo struct
type RouterSwapInfo struct {
	ForNative     bool     `bson:"forNative,omitempty"`
	ForUnderlying bool     `bson:"forUnderlying,omitempty"`
	Token         string   `bson:"token"`
	TokenID       string   `bson:"tokenID"`
	Path          []string `bson:"path,omitempty"`
	AmountOutMin  string   `bson:"amountOutMin,omitempty"`
	FromChainID   string   `bson:"fromChainID"`
	ToChainID     string   `bson:"toChainID"`
}

// AnyCallSwapInfo struct
type AnyCallSwapInfo struct {
	CallFrom        string   `bson:"callFrom"`
	CallTo          []string `bson:"callTo"`
	CallData        []string `bson:"callData"`
	Callbacks       []string `bson:"callbacks"`
	CallNonces      []string `bson:"callNonces"`
	CallFromChainID string   `bson:"callFromChainID"`
	CallToChainID   string   `bson:"callToChainID"`
}

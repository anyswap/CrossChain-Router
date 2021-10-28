package mongodb

// MgoSwap registered swap
type MgoSwap struct {
	Key         string `bson:"_id"` // fromChainID + txid + logindex
	SwapType    uint32 `bson:"swaptype"`
	TxID        string `bson:"txid"`
	TxTo        string `bson:"txto"`
	From        string `bson:"from"`
	Bind        string `bson:"bind"`
	LogIndex    int    `bson:"logIndex"`
	FromChainID string `bson:"fromChainID"`
	ToChainID   string `bson:"toChainID"`
	SwapInfo    `bson:"swapinfo"`
	Status      SwapStatus `bson:"status"`
	InitTime    int64      `bson:"inittime"`
	Timestamp   int64      `bson:"timestamp"`
	Memo        string     `bson:"memo"`
}

// ToSwapResult converts
func (swap *MgoSwap) ToSwapResult() *MgoSwapResult {
	return &MgoSwapResult{
		Key:         swap.Key,
		SwapType:    swap.SwapType,
		TxID:        swap.TxID,
		TxTo:        swap.TxTo,
		Bind:        swap.Bind,
		LogIndex:    swap.LogIndex,
		FromChainID: swap.FromChainID,
		ToChainID:   swap.ToChainID,
		SwapInfo:    swap.SwapInfo,
		Status:      swap.Status,
		InitTime:    swap.InitTime,
		Timestamp:   swap.Timestamp,
		Memo:        swap.Memo,
	}
}

// MgoSwapResult swap result (verified swap)
type MgoSwapResult struct {
	Key         string `bson:"_id"` // fromChainID + txid + logindex
	SwapType    uint32 `bson:"swaptype"`
	TxID        string `bson:"txid"`
	TxTo        string `bson:"txto"`
	TxHeight    uint64 `bson:"txheight"`
	TxTime      uint64 `bson:"txtime"`
	From        string `bson:"from"`
	To          string `bson:"to"`
	Bind        string `bson:"bind"`
	Value       string `bson:"value"`
	LogIndex    int    `bson:"logIndex"`
	FromChainID string `bson:"fromChainID"`
	ToChainID   string `bson:"toChainID"`
	SwapInfo    `bson:"swapinfo"`
	SwapTx      string     `bson:"swaptx"`
	OldSwapTxs  []string   `bson:"oldswaptxs,omitempty" json:"oldswaptxs,omitempty"`
	SwapHeight  uint64     `bson:"swapheight"`
	SwapTime    uint64     `bson:"swaptime"`
	SwapValue   string     `bson:"swapvalue"`
	SwapNonce   uint64     `bson:"swapnonce"`
	Status      SwapStatus `bson:"status"`
	InitTime    int64      `bson:"inittime"`
	Timestamp   int64      `bson:"timestamp"`
	Memo        string     `bson:"memo"`
	MPC         string     `bson:"mpc"`
}

// MgoUsedRValue security enhancement
type MgoUsedRValue struct {
	Key       string `bson:"_id"` // r + pubkey
	Timestamp int64  `bson:"timestamp"`
}

// SwapResultUpdateItems swap update items
type SwapResultUpdateItems struct {
	MPC        string
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
	ERC20SwapInfo   *ERC20SwapInfo   `bson:"routerSwapInfo,omitempty"  json:"routerSwapInfo,omitempty"`
	NFTSwapInfo     *NFTSwapInfo     `bson:"nftSwapInfo,omitempty"     json:"nftSwapInfo,omitempty"`
	AnyCallSwapInfo *AnyCallSwapInfo `bson:"anycallSwapInfo,omitempty" json:"anycallSwapInfo,omitempty"`
}

// ERC20SwapInfo struct
type ERC20SwapInfo struct {
	ForNative     bool     `bson:"forNative,omitempty"     json:"forNative,omitempty"`
	ForUnderlying bool     `bson:"forUnderlying,omitempty" json:"forUnderlying,omitempty"`
	Token         string   `bson:"token"                   json:"token"`
	TokenID       string   `bson:"tokenID"                 json:"tokenID"`
	Path          []string `bson:"path,omitempty"          json:"path,omitempty"`
	AmountOutMin  string   `bson:"amountOutMin,omitempty"  json:"amountOutMin,omitempty"`
}

// NFTSwapInfo struct
type NFTSwapInfo struct {
	Token   string   `bson:"token"          json:"token"`
	TokenID string   `bson:"tokenID"        json:"tokenID"`
	IDs     []string `bson:"ids"            json:"ids"`
	Amounts []string `bson:"amounts"        json:"amounts"`
	Batch   bool     `bson:"batch"          json:"batch"`
	Data    string   `bson:"data,omitempty" json:"data,omitempty"`
}

// AnyCallSwapInfo struct
type AnyCallSwapInfo struct {
	CallFrom   string   `bson:"callFrom"   json:"callFrom"`
	CallTo     []string `bson:"callTo"     json:"callTo"`
	CallData   []string `bson:"callData"   json:"callData"`
	Callbacks  []string `bson:"callbacks"  json:"callbacks"`
	CallNonces []string `bson:"callNonces" json:"callNonces"`
}

// GetToken get token
func (s *SwapInfo) GetToken() string {
	if s.ERC20SwapInfo != nil {
		return s.ERC20SwapInfo.Token
	}
	if s.NFTSwapInfo != nil {
		return s.NFTSwapInfo.Token
	}
	return ""
}

// GetTokenID get tokenID
func (s *SwapInfo) GetTokenID() string {
	if s.ERC20SwapInfo != nil {
		return s.ERC20SwapInfo.TokenID
	}
	if s.NFTSwapInfo != nil {
		return s.NFTSwapInfo.TokenID
	}
	return ""
}

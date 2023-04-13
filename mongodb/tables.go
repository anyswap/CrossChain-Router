package mongodb

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// MgoSwap registered swap
type MgoSwap struct {
	Key         string `bson:"_id" json:",omitempty"` // fromChainID + txid + logindex
	SwapType    uint32 `bson:"swaptype"`
	TxID        string `bson:"txid"`
	TxTo        string `bson:"txto"`
	TxHeight    uint64 `bson:"txheight"`
	From        string `bson:"from"`
	Bind        string `bson:"bind"`
	Value       string `bson:"value"`
	LogIndex    int    `bson:"logIndex"`
	FromChainID string `bson:"fromChainID"`
	ToChainID   string `bson:"toChainID"`
	SwapInfo    `bson:"swapinfo"`
	Status      SwapStatus `bson:"status"`
	InitTime    int64      `bson:"inittime"`
	Timestamp   int64      `bson:"timestamp"`
	Memo        string     `bson:"memo" json:",omitempty"`
}

// IsValid is valid
func (swap *MgoSwap) IsValid() bool {
	swapType := tokens.SwapType(swap.SwapType)
	if !swapType.IsValidType() ||
		swap.FromChainID == "" || swap.ToChainID == "" ||
		swap.TxID == "" || swap.Value == "" {
		return false
	}
	switch swapType {
	case tokens.ERC20SwapType, tokens.ERC20SwapTypeMixPool:
		if swap.ERC20SwapInfo == nil || swap.From == "" || swap.Bind == "" {
			return false
		}
	case tokens.NFTSwapType:
		if swap.NFTSwapInfo == nil || swap.From == "" || swap.Bind == "" {
			return false
		}
	case tokens.AnyCallSwapType:
		if swap.AnyCallSwapInfo == nil {
			return false
		}
	case tokens.SapphireRPCType: // internal usage, never store
		return false
	default:
		return false
	}

	return true
}

// ToSwapResult converts
func (swap *MgoSwap) ToSwapResult() *MgoSwapResult {
	return &MgoSwapResult{
		Key:         swap.Key,
		SwapType:    swap.SwapType,
		TxID:        swap.TxID,
		TxTo:        swap.TxTo,
		TxHeight:    swap.TxHeight,
		From:        swap.From,
		Bind:        swap.Bind,
		Value:       swap.Value,
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
	TxTime      uint64 `bson:"txtime" json:",omitempty"`
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
	Memo        string     `bson:"memo" json:",omitempty"`
	MPC         string     `bson:"mpc"`
	TTL         uint64     `bson:"ttl"`
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
	SwapHeight uint64
	SwapTime   uint64
	SwapValue  string
	SwapNonce  uint64
	Status     SwapStatus
	Timestamp  int64
	Memo       string
	TTL        uint64
}

// SwapInfo struct
type SwapInfo struct {
	ERC20SwapInfo   *ERC20SwapInfo   `bson:"routerSwapInfo,omitempty"  json:"routerSwapInfo,omitempty"`
	NFTSwapInfo     *NFTSwapInfo     `bson:"nftSwapInfo,omitempty"     json:"nftSwapInfo,omitempty"`
	AnyCallSwapInfo *AnyCallSwapInfo `bson:"anycallSwapInfo2,omitempty" json:"anycallSwapInfo2,omitempty"`
}

// ERC20SwapInfo struct
type ERC20SwapInfo struct {
	Token     string `bson:"token"                   json:"token"`
	TokenID   string `bson:"tokenID"                 json:"tokenID"`
	SwapoutID string `bson:"swapoutID,omitempty"     json:"swapoutID,omitempty"`
	CallProxy string `bson:"callProxy,omitempty"     json:"callProxy,omitempty"`
	CallData  string `bson:"callData,omitempty"      json:"callData,omitempty"`
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
	CallFrom    string `bson:",omitempty" json:"callFrom,omitempty"`
	CallTo      string `bson:",omitempty" json:"callTo,omitempty"`
	CallData    string `bson:",omitempty" json:"callData,omitempty"`
	Fallback    string `bson:",omitempty" json:"fallback,omitempty"`
	Flags       string `bson:",omitempty" json:"flags,omitempty"`
	AppID       string `bson:",omitempty" json:"appid,omitempty"`
	Nonce       string `bson:",omitempty" json:"nonce,omitempty"`
	ExtData     string `bson:",omitempty" json:"extdata,omitempty"`
	Message     string `bson:",omitempty" json:"message,omitempty"`
	Attestation string `bson:",omitempty" json:"attestation,omitempty"`
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

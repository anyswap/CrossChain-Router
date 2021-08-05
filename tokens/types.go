package tokens

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
)

// SwapType type
type SwapType uint32

// SwapType constants
const (
	RouterSwapType SwapType = iota + 1
	AnyCallSwapType
)

func (s SwapType) String() string {
	switch s {
	case RouterSwapType:
		return "routerswap"
	case AnyCallSwapType:
		return "anycallswap"
	default:
		return fmt.Sprintf("unknown swap type %d", s)
	}
}

// RouterSwapInfo struct
type RouterSwapInfo struct {
	ForNative     bool     `json:"forNative,omitempty"`
	ForUnderlying bool     `json:"forUnderlying,omitempty"`
	Token         string   `json:"token"`
	TokenID       string   `json:"tokenID"`
	Path          []string `json:"path,omitempty"`
	AmountOutMin  *big.Int `json:"amountOutMin,omitempty"`
}

// AnyCallSwapInfo struct
type AnyCallSwapInfo struct {
	CallFrom   string          `json:"callFrom"`
	CallTo     []string        `json:"callTo"`
	CallData   []hexutil.Bytes `json:"callData"`
	Callbacks  []string        `json:"callbacks"`
	CallNonces []*big.Int      `json:"callNonces"`
}

// SwapInfo struct
type SwapInfo struct {
	*RouterSwapInfo  `json:"routerSwapInfo,omitempty"`
	*AnyCallSwapInfo `json:"anycallSwapInfo,omitempty"`
}

// SwapTxInfo struct
type SwapTxInfo struct {
	SwapInfo    `json:"swapinfo"`
	SwapType    SwapType `json:"swaptype"`
	Hash        string   `json:"hash"`
	Height      uint64   `json:"height"`
	Timestamp   uint64   `json:"timestamp"`
	From        string   `json:"from"`
	TxTo        string   `json:"txto"`
	To          string   `json:"to"`
	Bind        string   `json:"bind"`
	Value       *big.Int `json:"value"`
	LogIndex    int      `json:"logIndex"`
	FromChainID *big.Int `json:"fromChainID"`
	ToChainID   *big.Int `json:"toChainID"`
}

// TxStatus struct
type TxStatus struct {
	Receipt       interface{} `json:"receipt,omitempty"`
	Confirmations uint64      `json:"confirmations"`
	BlockHeight   uint64      `json:"blockHeight"`
	BlockHash     string      `json:"blockHash"`
	BlockTime     uint64      `json:"blockTime"`
}

// VerifyArgs struct
type VerifyArgs struct {
	SwapType      SwapType `json:"swaptype,omitempty"`
	LogIndex      int      `json:"logIndex,omitempty"`
	AllowUnstable bool     `json:"allowUnstable,omitempty"`
}

// RegisterArgs struct
type RegisterArgs struct {
	SwapType SwapType `json:"swaptype,omitempty"`
	LogIndex int      `json:"logIndex,omitempty"`
}

// SwapArgs struct
type SwapArgs struct {
	SwapInfo    `json:"swapinfo"`
	Identifier  string   `json:"identifier,omitempty"`
	SwapID      string   `json:"swapid,omitempty"`
	SwapType    SwapType `json:"swaptype,omitempty"`
	Bind        string   `json:"bind,omitempty"`
	LogIndex    int      `json:"logIndex"`
	FromChainID *big.Int `json:"fromChainID"`
	ToChainID   *big.Int `json:"toChainID"`
}

// BuildTxArgs struct
type BuildTxArgs struct {
	SwapArgs    `json:"swapArgs,omitempty"`
	From        string         `json:"from,omitempty"`
	To          string         `json:"to,omitempty"`
	Value       *big.Int       `json:"value,omitempty"`
	OriginValue *big.Int       `json:"originValue,omitempty"`
	SwapValue   *big.Int       `json:"swapValue,omitempty"`
	Memo        string         `json:"memo,omitempty"`
	Input       *hexutil.Bytes `json:"input,omitempty"`
	Extra       *AllExtras     `json:"extra,omitempty"`
	ReplaceNum  uint64         `json:"replaceNum,omitempty"`
}

// AllExtras struct
type AllExtras struct {
	EthExtra *EthExtraArgs `json:"ethExtra,omitempty"`
}

// EthExtraArgs struct
type EthExtraArgs struct {
	Gas       *uint64  `json:"gas,omitempty"`
	GasPrice  *big.Int `json:"gasPrice,omitempty"`
	GasTipCap *big.Int `json:"gasTipCap,omitempty"`
	GasFeeCap *big.Int `json:"gasFeeCap,omitempty"`
	Nonce     *uint64  `json:"nonce,omitempty"`
	Deadline  int64    `json:"deadline,omitempty"`
}

// GetExtraArgs get extra args
func (args *BuildTxArgs) GetExtraArgs() *BuildTxArgs {
	return &BuildTxArgs{
		SwapArgs: args.SwapArgs,
		Extra:    args.Extra,
	}
}

// GetTxNonce get tx nonce
func (args *BuildTxArgs) GetTxNonce() uint64 {
	if args.Extra != nil && args.Extra.EthExtra != nil && args.Extra.EthExtra.Nonce != nil {
		return *args.Extra.EthExtra.Nonce
	}
	return 0
}

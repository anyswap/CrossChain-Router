package tokens

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/common/hexutil"
)

// constants
const (
	ReplaceSwapIdentifier = "replaceswap"
)

// SwapType type
type SwapType uint32

// SwapType constants
const (
	NonSwapType SwapType = iota
	RouterSwapType
)

func (s SwapType) String() string {
	switch s {
	case NonSwapType:
		return "nonswap"
	case RouterSwapType:
		return "routerswap"
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
	FromChainID   *big.Int `json:"fromChainID"`
	ToChainID     *big.Int `json:"toChainID"`
	LogIndex      int      `json:"logIndex"`
}

// SwapTxInfo struct
type SwapTxInfo struct {
	*RouterSwapInfo `json:"routerSwapInfo,omitempty"`

	SwapType  SwapType `json:"swaptype"`
	Hash      string   `json:"hash"`
	Height    uint64   `json:"height"`
	Timestamp uint64   `json:"timestamp"`
	From      string   `json:"from"`
	TxTo      string   `json:"txto"`
	To        string   `json:"to"`
	Bind      string   `json:"bind"`
	Value     *big.Int `json:"value"`
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
	*RouterSwapInfo `json:"routerSwapInfo,omitempty"`

	Identifier string   `json:"identifier,omitempty"`
	SwapID     string   `json:"swapid,omitempty"`
	SwapType   SwapType `json:"swaptype,omitempty"`
	Bind       string   `json:"bind,omitempty"`
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
}

// AllExtras struct
type AllExtras struct {
	EthExtra *EthExtraArgs `json:"ethExtra,omitempty"`
}

// EthExtraArgs struct
type EthExtraArgs struct {
	Gas      *uint64  `json:"gas,omitempty"`
	GasPrice *big.Int `json:"gasPrice,omitempty"`
	Nonce    *uint64  `json:"nonce,omitempty"`
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

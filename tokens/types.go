package tokens

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

// SwapType type
type SwapType uint32

// SwapType constants
const (
	UnknownSwapType SwapType = iota
	ERC20SwapType
	NFTSwapType

	MaxValidSwapType
	AnyCallSwapType
)

func (s SwapType) String() string {
	switch s {
	case ERC20SwapType:
		return "erc20swap"
	case NFTSwapType:
		return "nftswap"
	case AnyCallSwapType:
		return "anycallswap"
	default:
		return "unknownswap"
	}
}

// IsValidType is valid swap type
func (s SwapType) IsValidType() bool {
	return s > UnknownSwapType && s < MaxValidSwapType
}

// ERC20SwapInfo struct
type ERC20SwapInfo struct {
	ForNative     bool     `json:"forNative,omitempty"`
	ForUnderlying bool     `json:"forUnderlying,omitempty"`
	Token         string   `json:"token"`
	TokenID       string   `json:"tokenID"`
	Path          []string `json:"path,omitempty"`
	AmountOutMin  *big.Int `json:"amountOutMin,omitempty"`
}

// NFTSwapInfo struct
type NFTSwapInfo struct {
	Token   string     `json:"token"`
	TokenID string     `json:"tokenID"`
	IDs     []*big.Int `json:"ids"`
	Amounts []*big.Int `json:"amounts"`
	Batch   bool       `json:"batch"`
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
	ERC20SwapInfo   *ERC20SwapInfo   `json:"routerSwapInfo,omitempty"`
	NFTSwapInfo     *NFTSwapInfo     `json:"nftSwapInfo,omitempty"`
	AnyCallSwapInfo *AnyCallSwapInfo `json:"anycallSwapInfo,omitempty"`
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

// IsSwapTxOnChainAndFailed to make failed of swaptx
func (s *TxStatus) IsSwapTxOnChainAndFailed() bool {
	if s == nil || s.BlockHeight == 0 {
		return false // not on chain
	}
	if s.Receipt != nil { // for eth-like blockchain
		receipt, ok := s.Receipt.(*types.RPCTxReceipt)
		if !ok || !receipt.IsStatusOk() || len(receipt.Logs) == 0 {
			return true
		}
	}
	return false
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
	Reswapping  bool     `json:"reswapping,omitempty"`
}

// BuildTxArgs struct
type BuildTxArgs struct {
	SwapArgs    `json:"swapArgs,omitempty"`
	From        string         `json:"from,omitempty"`
	To          string         `json:"to,omitempty"`
	OriginFrom  string         `json:"originFrom,omitempty"`
	OriginTxTo  string         `json:"originTxTo,omitempty"`
	OriginValue *big.Int       `json:"originValue,omitempty"`
	SwapValue   *big.Int       `json:"swapValue,omitempty"`
	Value       *big.Int       `json:"value,omitempty"`
	Memo        string         `json:"memo,omitempty"`
	Input       *hexutil.Bytes `json:"input,omitempty"`
	Extra       *AllExtras     `json:"extra,omitempty"`
}

// AllExtras struct
type AllExtras struct {
	EthExtra   *EthExtraArgs `json:"ethExtra,omitempty"`
	ReplaceNum uint64        `json:"replaceNum,omitempty"`
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

// GetReplaceNum get rplace swap count
func (args *BuildTxArgs) GetReplaceNum() uint64 {
	if args.Extra != nil {
		return args.Extra.ReplaceNum
	}
	return 0
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

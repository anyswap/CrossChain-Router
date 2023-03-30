package tokens

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
)

// SwapType type
type SwapType uint32

// SwapType constants
const (
	UnknownSwapType SwapType = iota
	ERC20SwapType
	NFTSwapType
	AnyCallSwapType

	// special flags, do not use in register
	ERC20SwapTypeMixPool
	SapphireRPCType

	MaxValidSwapType
)

// SwapSubType constants
const (
	CurveAnycallSubType = "curve"
	AnycallSubTypeV5    = "v5" // for curve
	AnycallSubTypeV6    = "v6" // for hundred
	AnycallSubTypeV7    = "v7" // add callback
)

// IsValidAnycallSubType is valid anycall subType
func IsValidAnycallSubType(subType string) bool {
	switch subType {
	case CurveAnycallSubType, AnycallSubTypeV5, AnycallSubTypeV6, AnycallSubTypeV7:
		return true
	default:
		return false
	}
}

func (s SwapType) String() string {
	switch s {
	case ERC20SwapType:
		return "erc20swap"
	case NFTSwapType:
		return "nftswap"
	case AnyCallSwapType:
		return "anycallswap"
	case ERC20SwapTypeMixPool:
		return "mixpool"
	case SapphireRPCType:
		return "sapphireRPCType"
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
	Token     string `json:"token"`
	TokenID   string `json:"tokenID"`
	SwapoutID string `json:"swapoutID,omitempty"`

	CallProxy string        `json:"callProxy,omitempty"`
	CallData  hexutil.Bytes `json:"callData,omitempty"`
}

// NFTSwapInfo struct
type NFTSwapInfo struct {
	Token   string        `json:"token"`
	TokenID string        `json:"tokenID"`
	IDs     []*big.Int    `json:"ids"`
	Amounts []*big.Int    `json:"amounts"`
	Batch   bool          `json:"batch"`
	Data    hexutil.Bytes `json:"data,omitempty"`
}

// AnyCallSwapInfo struct
type AnyCallSwapInfo struct {
	CallFrom string        `json:"callFrom"`
	CallTo   string        `json:"callTo"`
	CallData hexutil.Bytes `json:"callData"`
	Fallback string        `json:"fallback,omitempty"`
	Flags    string        `json:"flags,omitempty"`
	AppID    string        `json:"appid,omitempty"`
	Nonce    string        `json:"nonce,omitempty"`
	ExtData  hexutil.Bytes `json:"extdata,omitempty"`

	Message     hexutil.Bytes `json:"message,omitempty"`
	Attestation hexutil.Bytes `json:"attestation,omitempty"`
}

// SwapInfo struct
type SwapInfo struct {
	ERC20SwapInfo   *ERC20SwapInfo   `json:"routerSwapInfo,omitempty"`
	NFTSwapInfo     *NFTSwapInfo     `json:"nftSwapInfo,omitempty"`
	AnyCallSwapInfo *AnyCallSwapInfo `json:"anycallSwapInfo2,omitempty"`
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
	BlockHeight   uint64      `json:"block_height"`
	BlockHash     string      `json:"block_hash"`
	BlockTime     uint64      `json:"block_time,omitempty"`
}

// StatusInterface interface
type StatusInterface interface {
	IsStatusOk() bool
}

// IsSwapTxOnChain is tx onchain
func (s *TxStatus) IsSwapTxOnChain() bool {
	return s != nil && s.BlockHeight > 0
}

// IsSwapTxOnChainAndFailed to make failed of swaptx
func (s *TxStatus) IsSwapTxOnChainAndFailed() bool {
	if !s.IsSwapTxOnChain() {
		return false // not on chain
	}
	if status, ok := s.Receipt.(StatusInterface); ok {
		if !status.IsStatusOk() {
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
	Selector    string         `json:"selector,omitempty"`
	Input       *hexutil.Bytes `json:"input,omitempty"`
	Extra       *AllExtras     `json:"extra,omitempty"`
}

// AllExtras struct
type AllExtras struct {
	Gas         *uint64       `json:"gas,omitempty"`
	GasPrice    *big.Int      `json:"gasPrice,omitempty"`
	GasTipCap   *big.Int      `json:"gasTipCap,omitempty"`
	GasFeeCap   *big.Int      `json:"gasFeeCap,omitempty"`
	Sequence    *uint64       `json:"sequence,omitempty"`
	ReplaceNum  uint64        `json:"replaceNum,omitempty"`
	Fee         *string       `json:"fee,omitempty"`
	RawTx       hexutil.Bytes `json:"rawTx,omitempty"`
	BlockHash   *string       `json:"blockHash,omitempty"`
	BlockID     *string       `json:"blockID,omitempty"`
	BlockNumber *uint64       `json:"blockNumber,omitempty"`
	TTL         *uint64       `json:"ttl,omitempty"`
	BridgeFee   *big.Int      `json:"bridgeFee,omitempty"`
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
	swapArgs := args.SwapArgs
	if swapArgs.SwapInfo.AnyCallSwapInfo != nil {
		// message should be retrieved from receipt logs
		anycallInfo := *swapArgs.SwapInfo.AnyCallSwapInfo
		anycallInfo.Message = nil
		swapArgs.SwapInfo.AnyCallSwapInfo = &anycallInfo
	}
	return &BuildTxArgs{
		From:     args.From,
		SwapArgs: swapArgs,
		Extra:    args.Extra,
	}
}

// GetTxNonce get tx nonce
func (args *BuildTxArgs) GetTxNonce() uint64 {
	if args.Extra != nil && args.Extra.Sequence != nil {
		return *args.Extra.Sequence
	}
	return 0
}

// SetTxNonce set tx nonce
func (args *BuildTxArgs) SetTxNonce(nonce uint64) {
	if args.Extra != nil {
		args.Extra.Sequence = &nonce
	} else {
		args.Extra = &AllExtras{Sequence: &nonce}
	}
}

// GetUniqueSwapIdentifier get unique swap identifier
func (args *BuildTxArgs) GetUniqueSwapIdentifier() string {
	fromChainID := args.FromChainID
	swapID := args.SwapID
	logIndex := args.LogIndex
	if common.IsHexHash(swapID) {
		swapID = common.HexToHash(swapID).Hex()
	}
	return fmt.Sprintf("%v:%v:%v", fromChainID, swapID, logIndex)
}

package mongodb

import (
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/tokens"
)

// -----------------------------------------------
// swap status change graph
// symbol '--->' mean transfer only under checked condition (eg. manual process)
//
// -----------------------------------------------
// 1. swap register status change graph
//
// TxNotStable -> |- TxVerifyFailed    -> manual
//                |- TxWithWrongMemo   -> manual
//                |- TxWithWrongSender -> manual
//                |- TxWithWrongValue  -> manual
//                |- SwapInBlacklist   -> manual
//                |- TxIncompatible    -> manual
//                |- ManualMakeFail    -> manual
//                |- BindAddrIsContract-> manual
//                |- RPCQueryError     -> manual
//                |- TxWithBigValue        ---> TxNotSwapped
//                |- TxSenderNotRegistered ---> TxNotStable
//                |- TxNotSwapped -> |- TxSwapFailed -> manual
//                                   |- TxProcessed (->MatchTxNotStable)
// -----------------------------------------------
// 2. swap result status change graph
//
// TxWithWrongMemo -> manual
// TxWithBigValue        ---> MatchTxEmpty
// TxSenderNotRegistered ---> MatchTxEmpty
// MatchTxEmpty          -> | MatchTxNotStable -> |- MatchTxStable
//                                                |- MatchTxFailed -> manual
// -----------------------------------------------

// SwapStatus swap status
type SwapStatus uint16

// swap status values
const (
	TxNotStable           SwapStatus = iota // 0
	TxVerifyFailed                          // 1
	TxWithWrongSender                       // 2
	TxWithWrongValue                        // 3
	TxIncompatible                          // 4
	TxNotSwapped                            // 5
	TxSwapFailed                            // 6
	TxProcessed                             // 7
	MatchTxEmpty                            // 8
	MatchTxNotStable                        // 9
	MatchTxStable                           // 10
	TxWithWrongMemo                         // 11
	TxWithBigValue                          // 12
	TxSenderNotRegistered                   // 13
	MatchTxFailed                           // 14
	SwapInBlacklist                         // 15
	ManualMakeFail                          // 16
	BindAddrIsContract                      // 17
	RPCQueryError                           // 18
	TxWithWrongPath                         // 19
	MissTokenConfig                         // 20
	NoUnderlyingToken                       // 21

	EstimateGasFailed = 127
	KeepStatus        = 255
	Reswapping        = 256
)

// IsRegisteredOk is successfully registered
func (status SwapStatus) IsRegisteredOk() bool {
	switch status {
	case TxNotStable, TxNotSwapped, TxSwapFailed, TxProcessed, ManualMakeFail:
		return true
	default:
		return false
	}
}

// CanReswap can reswap
func (status SwapStatus) CanReswap() bool {
	switch status {
	case TxSwapFailed, TxProcessed, MatchTxFailed:
		return true
	default:
		return false
	}
}

// nolint:gocyclo // allow big simple switch
func (status SwapStatus) String() string {
	switch status {
	case TxNotStable:
		return "TxNotStable"
	case TxVerifyFailed:
		return "TxVerifyFailed"
	case TxWithWrongSender:
		return "TxWithWrongSender"
	case TxWithWrongValue:
		return "TxWithWrongValue"
	case TxIncompatible:
		return "TxIncompatible"
	case TxNotSwapped:
		return "TxNotSwapped"
	case TxSwapFailed:
		return "TxSwapFailed"
	case TxProcessed:
		return "TxProcessed"
	case MatchTxEmpty:
		return "MatchTxEmpty"
	case MatchTxNotStable:
		return "MatchTxNotStable"
	case MatchTxStable:
		return "MatchTxStable"
	case TxWithWrongMemo:
		return "TxWithWrongMemo"
	case TxWithBigValue:
		return "TxWithBigValue"
	case TxSenderNotRegistered:
		return "TxSenderNotRegistered"
	case MatchTxFailed:
		return "MatchTxFailed"
	case SwapInBlacklist:
		return "SwapInBlacklist"
	case ManualMakeFail:
		return "ManualMakeFail"
	case BindAddrIsContract:
		return "BindAddrIsContract"
	case RPCQueryError:
		return "RPCQueryError"
	case TxWithWrongPath:
		return "TxWithWrongPath"
	case MissTokenConfig:
		return "MissTokenConfig"
	case NoUnderlyingToken:
		return "NoUnderlyingToken"

	case EstimateGasFailed:
		return "EstimateGasFailed"
	case Reswapping:
		return "Reswapping"
	default:
		return fmt.Sprintf("unknown swap status %d", status)
	}
}

// GetRouterSwapStatusByVerifyError get router swap status by verify error
func GetRouterSwapStatusByVerifyError(err error) SwapStatus {
	if !tokens.ShouldRegisterRouterSwapForError(err) {
		return TxVerifyFailed
	}
	switch {
	case err == nil:
		return TxNotStable
	case errors.Is(err, tokens.ErrTxWithWrongValue):
		return TxWithWrongValue
	case errors.Is(err, tokens.ErrTxWithWrongPath):
		return TxWithWrongPath
	case errors.Is(err, tokens.ErrMissTokenConfig):
		return MissTokenConfig
	case errors.Is(err, tokens.ErrNoUnderlyingToken):
		return NoUnderlyingToken
	default:
		return TxVerifyFailed
	}
}

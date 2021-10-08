package mongodb

import (
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// -----------------------------------------------
// swap status change graph
// symbol '--->' mean transfer only under checked condition (eg. manual process)
//
// -----------------------------------------------
// 1. swap register status change graph
//
// TxNotStable -> |- TxVerifyFailed    -> manual
//                |- TxWithWrongValue  -> manual
//                |- SwapInBlacklist   -> manual
//                |- TxWithBigValue    ---> TxNotSwapped
//                |- TxNotSwapped -> |- TxProcessed (->MatchTxNotStable)
// -----------------------------------------------
// 2. swap result status change graph
//
// TxWithBigValue ---> MatchTxEmpty
// MatchTxEmpty   -> | MatchTxNotStable -> |- MatchTxStable
//                                         |- MatchTxFailed -> manual
// -----------------------------------------------

// SwapStatus swap status
type SwapStatus uint16

// swap status values
const (
	TxNotStable       SwapStatus = 0
	TxVerifyFailed    SwapStatus = 1
	TxWithWrongValue  SwapStatus = 3
	TxNotSwapped      SwapStatus = 5
	TxProcessed       SwapStatus = 7
	MatchTxEmpty      SwapStatus = 8
	MatchTxNotStable  SwapStatus = 9
	MatchTxStable     SwapStatus = 10
	TxWithBigValue    SwapStatus = 12
	MatchTxFailed     SwapStatus = 14
	SwapInBlacklist   SwapStatus = 15
	TxWithWrongPath   SwapStatus = 19
	MissTokenConfig   SwapStatus = 20
	NoUnderlyingToken SwapStatus = 21

	KeepStatus SwapStatus = 255
	Reswapping SwapStatus = 256
)

// IsResultStatus is swap result status
func (status SwapStatus) IsResultStatus() bool {
	switch status {
	case MatchTxEmpty, MatchTxNotStable, MatchTxStable, MatchTxFailed, Reswapping:
		return true
	default:
		return false
	}
}

// IsRegisteredOk is successfully registered
func (status SwapStatus) IsRegisteredOk() bool {
	switch status {
	case TxNotStable, TxNotSwapped, TxProcessed:
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
	case TxWithWrongValue:
		return "TxWithWrongValue"
	case TxNotSwapped:
		return "TxNotSwapped"
	case TxProcessed:
		return "TxProcessed"
	case MatchTxEmpty:
		return "MatchTxEmpty"
	case MatchTxNotStable:
		return "MatchTxNotStable"
	case MatchTxStable:
		return "MatchTxStable"
	case TxWithBigValue:
		return "TxWithBigValue"
	case MatchTxFailed:
		return "MatchTxFailed"
	case SwapInBlacklist:
		return "SwapInBlacklist"
	case TxWithWrongPath:
		return "TxWithWrongPath"
	case MissTokenConfig:
		return "MissTokenConfig"
	case NoUnderlyingToken:
		return "NoUnderlyingToken"

	case KeepStatus:
		return "KeepStatus"
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

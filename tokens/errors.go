package tokens

import (
	"errors"
)

// common errors
var (
	ErrNotImplemented        = errors.New("not implemented")
	ErrSwapTypeNotSupported  = errors.New("swap type not supported")
	ErrNoBridgeForChainID    = errors.New("no bridge for chain id")
	ErrSwapTradeNotSupport   = errors.New("swap trade not support")
	ErrNotFound              = errors.New("not found")
	ErrTxNotFound            = errors.New("tx not found")
	ErrTxNotStable           = errors.New("tx not stable")
	ErrLogIndexOutOfRange    = errors.New("log index out of range")
	ErrTxWithWrongReceipt    = errors.New("tx with wrong receipt")
	ErrTxWithWrongContract   = errors.New("tx with wrong contract")
	ErrTxWithWrongTopics     = errors.New("tx with wrong log topics")
	ErrSwapoutLogNotFound    = errors.New("swapout log not found or removed")
	ErrTxWithRemovedLog      = errors.New("tx with removed log")
	ErrWrongBindAddress      = errors.New("wrong bind address")
	ErrWrongRawTx            = errors.New("wrong raw tx")
	ErrUnsupportedFuncHash   = errors.New("unsupported method func hash")
	ErrWrongCountOfMsgHashes = errors.New("wrong count of msg hashed")
	ErrMsgHashMismatch       = errors.New("message hash mismatch")
	ErrSwapInBlacklist       = errors.New("swap is in black list")
	ErrTxBeforeInitialHeight = errors.New("transaction before initial block height")
	ErrEstimateGasFailed     = errors.New("estimate gas failed")
	ErrRPCQueryError         = errors.New("rpc query error")
	ErrMissDynamicFeeConfig  = errors.New("miss dynamic fee config")
	ErrFromChainIDMismatch   = errors.New("from chainID mismatch")
	ErrMissMPCPublicKey      = errors.New("miss mpc public key config")
	ErrMissRouterInfo        = errors.New("miss router info")
	ErrSenderMismatch        = errors.New("sender mismatch")
	ErrTxWithWrongSender     = errors.New("tx with wrong sender")
	// errors should register in router swap
	ErrTxWithWrongValue  = errors.New("tx with wrong value")
	ErrTxWithWrongPath   = errors.New("swap trade tx with wrong path")
	ErrMissTokenConfig   = errors.New("miss token config")
	ErrNoUnderlyingToken = errors.New("no underlying token")
)

// ShouldRegisterRouterSwapForError return true if this error should record in database
func ShouldRegisterRouterSwapForError(err error) bool {
	switch {
	case err == nil,
		errors.Is(err, ErrTxWithWrongValue),
		errors.Is(err, ErrTxWithWrongPath),
		errors.Is(err, ErrMissTokenConfig),
		errors.Is(err, ErrNoUnderlyingToken):
		return true
	}
	return false
}

// IsRPCQueryOrNotFoundError is rpc or not found error
func IsRPCQueryOrNotFoundError(err error) bool {
	return errors.Is(err, ErrRPCQueryError) || errors.Is(err, ErrNotFound)
}

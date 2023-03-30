package tokens

import (
	"errors"
)

// common errors
var (
	ErrNotImplemented         = errors.New("not implemented")
	ErrSwapTypeNotSupported   = errors.New("swap type not supported")
	ErrUnknownSwapSubType     = errors.New("unknown swap sub type")
	ErrNoBridgeForChainID     = errors.New("no bridge for chain id")
	ErrSwapTradeNotSupport    = errors.New("swap trade not support")
	ErrNonceNotSupport        = errors.New("nonce not support")
	ErrNotFound               = errors.New("not found")
	ErrTxNotFound             = errors.New("tx not found")
	ErrDepositNotFound        = errors.New("deposit not found")
	ErrTxNotStable            = errors.New("tx not stable")
	ErrLogIndexOutOfRange     = errors.New("log index out of range")
	ErrTxWithWrongReceipt     = errors.New("tx with wrong receipt")
	ErrTxWithWrongReceiver    = errors.New("tx with wrong receiver")
	ErrTxWithWrongContract    = errors.New("tx with wrong contract")
	ErrTxWithWrongTopics      = errors.New("tx with wrong log topics")
	ErrSwapoutLogNotFound     = errors.New("swapout log not found or removed")
	ErrSwapoutPatternMismatch = errors.New("swapout pattern mismatch")
	ErrTxWithRemovedLog       = errors.New("tx with removed log")
	ErrWrongBindAddress       = errors.New("wrong bind address")
	ErrWrongRawTx             = errors.New("wrong raw tx")
	ErrUnsupportedFuncHash    = errors.New("unsupported method func hash")
	ErrWrongCountOfMsgHashes  = errors.New("wrong count of msg hashed")
	ErrMsgHashMismatch        = errors.New("message hash mismatch")
	ErrSwapInBlacklist        = errors.New("swap is in black list")
	ErrTxBeforeInitialHeight  = errors.New("transaction before initial block height")
	ErrEstimateGasFailed      = errors.New("estimate gas failed")
	ErrRPCQueryError          = errors.New("rpc query error")
	ErrMissDynamicFeeConfig   = errors.New("miss dynamic fee config")
	ErrFromChainIDMismatch    = errors.New("from chainID mismatch")
	ErrSameFromAndToChainID   = errors.New("from and to chainID are same")
	ErrMissMPCPublicKey       = errors.New("miss mpc public key config")
	ErrMissRouterInfo         = errors.New("miss router info")
	ErrRouterVersionMismatch  = errors.New("router version mismatch")
	ErrSenderMismatch         = errors.New("sender mismatch")
	ErrTxWithWrongSender      = errors.New("tx with wrong sender")
	ErrToChainIDMismatch      = errors.New("to chainID mismatch")
	ErrTxWithWrongStatus      = errors.New("tx with wrong status")
	ErrUnknownSwapoutType     = errors.New("unknown swapout type")
	ErrEmptyTokenID           = errors.New("empty tokenID")
	ErrNoEnoughReserveBudget  = errors.New("no enough reserve budget")
	ErrTxWithNoPayment        = errors.New("tx with no payment")
	ErrTxIsNotValidated       = errors.New("tx is not validated")
	ErrPauseSwapInto          = errors.New("maintain: pause swap into")
	ErrBuildTxErrorAndDelay   = errors.New("[build tx error]")
	ErrSwapoutIDNotExist      = errors.New("swapoutID not exist")
	ErrValidPublicKey         = errors.New("valid public key error")
	ErrBroadcastTx            = errors.New("broadcast tx error")
	ErrSimulateTx             = errors.New("simulate tx error")
	ErrTxWithWrongMemo        = errors.New("tx with wrong memo")
	ErrFallbackNotSupport     = errors.New("app does not support fallback")
	ErrQueryTokenBalance      = errors.New("query token balance error")
	ErrTokenBalanceNotEnough  = errors.New("token balance not enough")
	ErrGetLatestBlockNumber   = errors.New("get latest block number error")
	ErrGetAccountNonce        = errors.New("get account nonce error")
	ErrGetUnderlying          = errors.New("get underlying address error")
	ErrGetMPC                 = errors.New("get mpc address error")
	ErrTokenDecimals          = errors.New("get token decimals error")
	ErrGetLatestBlockHash     = errors.New("get latest block hash error")
	ErrTxResultType           = errors.New("tx type is not TransactionResult")
	ErrGetNodeInfo            = errors.New("err to get node info")
	ErrPayloadType            = errors.New("payload type error")
	ErrGetOutPutIDs           = errors.New("get output id error")
	ErrGetOutPutByID          = errors.New("get output by id error")
	ErrCommitMessage          = errors.New("commit message error")
	ErrProofOfWork            = errors.New("proof of work error")
	ErrBalanceNoKeepAlive     = errors.New("balance can't keep alive")
	ErrSwapValueTooLess       = errors.New("swap value must bigger than 1000000")
	ErrCheckBalance           = errors.New("check balance error")
	ErrInputAndOutputLength   = errors.New("input and output must bigger than one")
	ErrTxWithWrongAssetLength = errors.New("tx with wrong asset length")
	ErrOutputLength           = errors.New("output lenght is zero")
	ErrMpcAddrMissMatch       = errors.New("receiver addr not match mpc addr")
	ErrMetadataKeyMissMatch   = errors.New("metadata key not match 123")
	ErrAdaSwapOutAmount       = errors.New("swap ada amount too small")
	ErrTokenBalancesNotEnough = errors.New("token balance not enough")
	ErrBalanceNotEnough       = errors.New("balance not enough")
	ErrAdaBalancesNotEnough   = errors.New("ada balance not enough")
	ErrOutputIndexSort        = errors.New("output not order by index asc")
	ErrCmdArgVerify           = errors.New("cmd args verify fails")
	ErrAggregateTx            = errors.New("aggregate tx fails")
	ErrNilSwapValue           = errors.New("swap value is nil")
	ErrMessageSentNotFound    = errors.New("message sent not found")
	ErrNoAttestationServer    = errors.New("no attesttation server")
	ErrGetAttestationFailed   = errors.New("get attesttation failed")
	ErrTxWithoutSigner        = errors.New("tx without signer")
	ErrReswapNotSupport       = errors.New("reswap not support")
	ErrTxWithZeroValue        = errors.New("tx with zero value")
	ErrGetBlockNumberByID     = errors.New("get block number by id error")
	ErrSendTx                 = errors.New("send tx fails")
	ErrGetAccount             = errors.New("get account fails")
)

// errors should register in router swap
var (
	ErrTxWithWrongValue  = errors.New("tx with wrong value")
	ErrTxWithWrongPath   = errors.New("swap trade tx with wrong path")
	ErrMissTokenConfig   = errors.New("miss token config")
	ErrNoUnderlyingToken = errors.New("no underlying token")
	ErrVerifyTxUnsafe    = errors.New("[tx maybe unsafe]")
	ErrSwapoutForbidden  = errors.New("swapout forbidden")
)

// ShouldRegisterRouterSwapForError return true if this error should record in database
func ShouldRegisterRouterSwapForError(err error) bool {
	switch {
	case err == nil,
		errors.Is(err, ErrTxWithWrongValue),
		errors.Is(err, ErrTxWithWrongPath),
		errors.Is(err, ErrMissTokenConfig),
		errors.Is(err, ErrNoUnderlyingToken),
		errors.Is(err, ErrVerifyTxUnsafe),
		errors.Is(err, ErrSwapoutForbidden):
		return true
	}
	return false
}

// IsRPCQueryOrNotFoundError is rpc or not found error
func IsRPCQueryOrNotFoundError(err error) bool {
	return errors.Is(err, ErrRPCQueryError) || errors.Is(err, ErrNotFound)
}

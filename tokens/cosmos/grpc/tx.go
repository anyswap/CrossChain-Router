package grpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/mempool"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos/grpc/retry"
)

var (
	txTimeout            = time.Minute
	txStatusPollInterval = 500 * time.Millisecond

	txNextBlocksTimeout      = time.Minute
	txNextBlocksPollInterval = time.Second

	requestTimeout = 10 * time.Second
)

// Factory is a re-export of the cosmos sdk tx.Factory type, to make usage of this package more convenient.
// It will help users by removing the need to import tx package from cosmos sdk and help avoid package name collision.
type Factory = tx.Factory

// Sign signs a given tx with a named key. The bytes signed over are canonical.
// The resulting signature will be added to the transaction builder overwriting the previous
// ones if overwrite=true (otherwise, the signature will be appended).
// Signing a transaction with mutltiple signers in the DIRECT mode is not supprted and will
// return an error.
// An error is returned upon failure.
// https://github.com/cosmos/cosmos-sdk/blob/v0.45.2/client/tx/tx.go
var Sign = tx.Sign

// BroadcastTx attempts to generate, sign and broadcast a transaction with the
// given set of messages. It will return an error upon failure.
// NOTE: copied from the link below and made some changes.
// the main idea is to add context.Context to the signature and use it
// https://github.com/cosmos/cosmos-sdk/blob/v0.45.2/client/tx/tx.go
// TODO: add test to check if client respects ctx.
func BroadcastTx(ctx context.Context, clientCtx ClientContext, txf Factory, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	txf, err := prepareFactory(ctx, clientCtx, txf)
	if err != nil {
		return nil, err
	}

	if txf.SimulateAndExecute() {
		gasPrice, err := GetGasPrice(ctx, clientCtx)
		if err != nil {
			return nil, err
		}
		txf = txf.WithGasPrices(gasPrice.String())

		_, adjusted, err := CalculateGas(ctx, clientCtx, txf, msgs...)
		if err != nil {
			return nil, err
		}

		txf = txf.WithGas(adjusted)
	}

	unsignedTx, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	unsignedTx.SetFeeGranter(clientCtx.FeeGranterAddress())

	// in case the name is not provided by that address, take the name by the address
	fromName := clientCtx.FromName()
	if fromName == "" && len(clientCtx.FromAddress()) > 0 {
		key, err := clientCtx.Keyring().KeyByAddress(clientCtx.FromAddress())
		if err != nil {
			return nil, errors.Errorf("failed to get key by the address %q from the keyring", clientCtx.FromAddress().String())
		}
		fromName = key.GetName()
	}

	err = tx.Sign(txf, fromName, unsignedTx, true)
	if err != nil {
		return nil, err
	}

	txBytes, err := clientCtx.TxConfig().TxEncoder()(unsignedTx.GetTx())
	if err != nil {
		return nil, err
	}

	return BroadcastRawTx(ctx, clientCtx, txBytes)
}

// CalculateGas simulates the execution of a transaction and returns the
// simulation response obtained by the query and the adjusted gas amount.
func CalculateGas(ctx context.Context, clientCtx ClientContext, txf Factory, msgs ...sdk.Msg) (*sdktx.SimulateResponse, uint64, error) {
	txBytes, err := tx.BuildSimTx(txf, msgs...)
	if err != nil {
		return nil, 0, err
	}

	txSvcClient := sdktx.NewServiceClient(clientCtx)
	simRes, err := txSvcClient.Simulate(ctx, &sdktx.SimulateRequest{
		TxBytes: txBytes,
	})
	if err != nil {
		return nil, 0, errors.Wrap(err, "transaction estimation failed")
	}

	if txf.GasAdjustment() == 0 {
		txf = txf.WithGasAdjustment(1.0)
	}

	return simRes, uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed)), nil
}

func SimulateTx(ctx context.Context, clientCtx ClientContext, txBytes []byte) (*sdktx.SimulateResponse, error) {
	txSvcClient := sdktx.NewServiceClient(clientCtx)
	simRes, err := txSvcClient.Simulate(ctx, &sdktx.SimulateRequest{
		TxBytes: txBytes,
	})
	if err != nil {
		return nil, errors.Wrap(err, "transaction estimation failed")
	}
	return simRes, nil
}

// BroadcastRawTx broadcast the txBytes using the clientCtx and set BroadcastMode.
func BroadcastRawTx(ctx context.Context, clientCtx ClientContext, txBytes []byte) (*sdk.TxResponse, error) {
	// broadcast to a Tendermint node
	switch clientCtx.BroadcastMode() {
	case flags.BroadcastSync:
		res, err := clientCtx.Client().BroadcastTxSync(ctx, txBytes)
		if err != nil {
			return nil, err
		}

		if res.Code != 0 {
			return nil, errors.Wrapf(sdkerrors.ABCIError(res.Codespace, res.Code, res.Log),
				"transaction '%s' failed", res.Hash.String())
		}

		return sdk.NewResponseFormatBroadcastTx(res), nil

	case flags.BroadcastAsync:
		res, err := clientCtx.Client().BroadcastTxAsync(ctx, txBytes)
		if err != nil {
			return nil, err
		}
		return sdk.NewResponseFormatBroadcastTx(res), nil

	case flags.BroadcastBlock:
		return broadcastTxCommit(ctx, clientCtx, txBytes)

	default:
		return nil, errors.Errorf("unsupported broadcast mode %s; supported types: sync, async, block", clientCtx.BroadcastMode())
	}
}

// broadcastTxCommit broadcasts encoded transaction, waits until it is included in a block
func broadcastTxCommit(ctx context.Context, clientCtx ClientContext, encodedTx []byte) (*sdk.TxResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	txHash := fmt.Sprintf("%X", tmtypes.Tx(encodedTx).Hash())
	res, err := clientCtx.Client().BroadcastTxSync(requestCtx, encodedTx)
	if err != nil {
		if errors.Is(err, requestCtx.Err()) {
			return nil, errors.WithStack(err)
		}

		if err := convertTendermintError(err); !sdkerrors.ErrTxInMempoolCache.Is(err) {
			return nil, errors.WithStack(err)
		}
	} else if res.Code != 0 {
		return nil, errors.Wrapf(sdkerrors.ABCIError(res.Codespace, res.Code, res.Log),
			"transaction '%s' failed", txHash)
	}

	awaitRes, err := AwaitTx(ctx, clientCtx, txHash)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return sdk.NewResponseResultTx(awaitRes, nil, ""), nil
}

func prepareFactory(ctx context.Context, clientCtx ClientContext, txf tx.Factory) (tx.Factory, error) {
	if txf.AccountNumber() == 0 && txf.Sequence() == 0 {
		acc, err := GetAccountInfo(ctx, clientCtx, clientCtx.FromAddress().String())
		if err != nil {
			return txf, err
		}
		txf = txf.
			WithAccountNumber(acc.GetAccountNumber()).
			WithSequence(acc.GetSequence())
	}

	return txf, nil
}

// AwaitTx awaits until a signed transaction is included in a block, returning the result.
func AwaitTx(
	ctx context.Context,
	clientCtx ClientContext,
	txHash string,
) (resultTx *coretypes.ResultTx, err error) {
	txHashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, errors.Wrap(err, "tx hash is not a valid hex")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, txTimeout)
	defer cancel()

	if err = retry.Do(timeoutCtx, txStatusPollInterval, func() error {
		requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		var err error
		resultTx, err = clientCtx.Client().Tx(requestCtx, txHashBytes, false)
		if err != nil {
			return retry.Retryable(errors.WithStack(err))
		}

		if resultTx.TxResult.Code != 0 {
			res := resultTx.TxResult
			return errors.Wrapf(sdkerrors.ABCIError(res.Codespace, res.Code, res.Log), "transaction '%s' failed", txHash)
		}

		if resultTx.Height == 0 {
			return retry.Retryable(errors.Errorf("transaction '%s' hasn't been included in a block yet", txHash))
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return resultTx, nil
}

// AwaitNextBlocks waits for next blocks.
func AwaitNextBlocks(
	ctx context.Context,
	clientCtx ClientContext,
	nextBlocks int64,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, txNextBlocksTimeout)
	defer cancel()

	heightToStart := int64(0)
	return retry.Do(timeoutCtx, txNextBlocksPollInterval, func() error {
		requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		res, err := clientCtx.Client().Status(requestCtx)
		if err != nil {
			return retry.Retryable(errors.WithStack(err))
		}

		currentHeight := res.SyncInfo.LatestBlockHeight
		if heightToStart == 0 {
			heightToStart = currentHeight
		}

		targetHeight := heightToStart + nextBlocks
		if currentHeight < targetHeight {
			return retry.Retryable(errors.Errorf("target block: %d hasn't been reached yet, current: %d", targetHeight, currentHeight))
		}

		return nil
	})
}

// GetGasPrice returns the current gas price of the chain
func GetGasPrice(
	ctx context.Context,
	clientCtx ClientContext,
) (sdk.DecCoin, error) {
	return sdk.NewInt64DecCoin("", 0), nil // TODO
}

// the idea behind this function is to map it similarly to how cosmos sdk does it in the link below
// so the users can match against cosmos sdk error types.
// https://github.com/cosmos/cosmos-sdk/blob/v0.45.2/client/broadcast.go#L49
func convertTendermintError(err error) error {
	if err == nil {
		return nil
	}
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, strings.ToLower(mempool.ErrTxInCache.Error())):
		return sdkerrors.ErrTxInMempoolCache.Wrap(err.Error())
	case strings.Contains(errStr, sdkerrors.ErrMempoolIsFull.Error()):
		return sdkerrors.ErrMempoolIsFull.Wrap(err.Error())
	case strings.Contains(errStr, sdkerrors.ErrTxTooLarge.Error()):
		return sdkerrors.ErrTxTooLarge.Wrap(err.Error())
	default:
		return err
	}
}

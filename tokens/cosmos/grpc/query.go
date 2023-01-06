package grpc

import (
	"context"
	"encoding/hex"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/pkg/errors"
)

func GetLatestBlockNumber(ctx context.Context, clientCtx ClientContext) (uint64, error) {
	res, err := clientCtx.Client().Status(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(res.SyncInfo.LatestBlockHeight), nil
}

func GetChainID(ctx context.Context, clientCtx ClientContext) (string, error) {
	status, err := clientCtx.Client().Status(ctx)
	if err != nil {
		return "", err
	}
	res, err := clientCtx.Client().Block(ctx, &status.SyncInfo.LatestBlockHeight)
	if err != nil {
		return "", err
	}
	return res.Block.Header.ChainID, nil
}

func GetTransactionByHash(ctx context.Context, clientCtx ClientContext, txHash string) (*sdk.TxResponse, error) {
	txHashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, errors.Wrap(err, "tx hash is not a valid hex")
	}
	txres, err := clientCtx.Client().Tx(ctx, txHashBytes, false)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var tx sdktx.Tx
	err = protoCodec.Unmarshal(txres.Tx, &tx) //TODO
	if err != nil {
		return nil, errors.WithStack(err)
	}
	anyTx, err := codectypes.NewAnyWithValue(&tx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return sdk.NewResponseResultTx(txres, anyTx, ""), nil
}

func GetDenomBalance(
	ctx context.Context,
	clientCtx ClientContext,
	address, denom string,
) (sdk.Int, error) {
	bankClient := banktypes.NewQueryClient(clientCtx)
	res, err := bankClient.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: address,
		Denom:   denom,
	})
	if err != nil {
		return sdk.ZeroInt(), errors.WithStack(err)
	}
	return res.Balance.Amount, nil
}

// GetAccountInfo returns account number and account sequence for provided address
func GetAccountInfo(
	ctx context.Context,
	clientCtx ClientContext,
	address string,
) (authtypes.AccountI, error) {
	authClient := authtypes.NewQueryClient(clientCtx)
	res, err := authClient.Account(ctx,
		&authtypes.QueryAccountRequest{
			Address: address,
		},
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var acc authtypes.AccountI
	if err := clientCtx.InterfaceRegistry().UnpackAny(res.Account, &acc); err != nil {
		return nil, errors.WithStack(err)
	}

	return acc, nil
}

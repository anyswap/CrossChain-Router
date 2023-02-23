package grpc

import (
	"context"
	"encoding/hex"

	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/pkg/errors"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
)

var protoCodec = encoding.GetCodec(proto.Name)

func GetLatestBlockNumber(ctx context.Context, clientCtx cosmosClient.Context) (uint64, error) {
	res, err := clientCtx.Client.Status(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(res.SyncInfo.LatestBlockHeight), nil
}

func GetChainID(ctx context.Context, clientCtx cosmosClient.Context) (string, error) {
	status, err := clientCtx.Client.Status(ctx)
	if err != nil {
		return "", err
	}
	res, err := clientCtx.Client.Block(ctx, &status.SyncInfo.LatestBlockHeight)
	if err != nil {
		return "", err
	}
	return res.Block.Header.ChainID, nil
}

func GetTransactionByHash(ctx context.Context, clientCtx cosmosClient.Context, txHash string) (*sdk.TxResponse, error) {
	txHashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, errors.Wrap(err, "tx hash is not a valid hex")
	}
	txres, err := clientCtx.Client.Tx(ctx, txHashBytes, false)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var tx sdktx.Tx
	err = protoCodec.Unmarshal(txres.Tx, &tx)
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
	clientCtx cosmosClient.Context,
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
	clientCtx cosmosClient.Context,
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
	if err := clientCtx.InterfaceRegistry.UnpackAny(res.Account, &acc); err != nil {
		return nil, errors.WithStack(err)
	}

	return acc, nil
}

func SimulateTx(
	ctx context.Context,
	clientCtx cosmosClient.Context,
	txBytes []byte,
) (*sdktx.SimulateResponse, error) {
	txSvcClient := sdktx.NewServiceClient(clientCtx)
	simRes, err := txSvcClient.Simulate(ctx, &sdktx.SimulateRequest{
		TxBytes: txBytes,
	})
	if err != nil {
		return nil, errors.Wrap(err, "transaction estimation failed")
	}
	return simRes, nil
}

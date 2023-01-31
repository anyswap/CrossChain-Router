package cosmos

import (
	"context"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos/grpc"
	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/pkg/errors"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

var (
	rpcClients    []rpcclient.Client
	rpcClientsMap = make(map[string]rpcclient.Client)

	ctx = context.Background()
)

func (b *Bridge) initGrpcClients() {
	for _, url := range b.GatewayConfig.GRPCAPIAddress {
		rpcClient, err := cosmosclient.NewClientFromNode(url)
		if err != nil {
			log.Warn("new grpc client failed", "url", url, "err", err)
			continue
		}
		rpcClients = append(rpcClients, rpcClient)
		rpcClientsMap[url] = rpcClient
	}
	if len(rpcClients) > 0 {
		log.Info("init grpc clients success", "count", len(rpcClients))
	}
}

func (b *Bridge) GRPCGetLatestBlockNumber() (res uint64, err error) {
	for _, rpcClient := range rpcClients {
		clientCtx := b.ClientContext.WithClient(rpcClient)
		res, err = grpc.GetLatestBlockNumber(ctx, clientCtx)
		if err == nil {
			return res, nil
		}
	}
	if err != nil {
		log.Warn("GRPCGetLatestBlockNumber failed", "err", err)
	}
	return 0, wrapRPCQueryError(err, "GRPCGetLatestBlockNumber")
}

func (b *Bridge) GRPCGetLatestBlockNumberOf(url string) (res uint64, err error) {
	rpcClient, exist := rpcClientsMap[url]
	if !exist {
		rpcClient, err = cosmosclient.NewClientFromNode(url)
		if err != nil {
			log.Warn("new grpc client failed", "url", url, "err", err)
			return 0, err
		}
	}
	clientCtx := b.ClientContext.WithClient(rpcClient)
	res, err = grpc.GetLatestBlockNumber(ctx, clientCtx)
	if err == nil {
		return res, nil
	}
	if err != nil && exist {
		log.Warn("GRPCGetLatestBlockNumber failed", "err", err)
	}
	return 0, wrapRPCQueryError(err, "GRPCGetLatestBlockNumber")
}

func (b *Bridge) GRPCGetChainID() (res string, err error) {
	for _, rpcClient := range rpcClients {
		clientCtx := b.ClientContext.WithClient(rpcClient)
		res, err = grpc.GetChainID(ctx, clientCtx)
		if err == nil {
			return res, nil
		}
	}
	if err != nil {
		log.Warn("GRPCGetChainID failed", "err", err)
	}
	return "", wrapRPCQueryError(err, "GRPCGetChainID")
}

func (b *Bridge) GRPCGetTransactionByHash(txHash string) (res *GetTxResponse, err error) {
	var txres *sdk.TxResponse
	for _, rpcClient := range rpcClients {
		clientCtx := b.ClientContext.WithClient(rpcClient)
		txres, err = grpc.GetTransactionByHash(ctx, clientCtx, txHash)
		if err == nil {
			var tx *sdktx.Tx
			if err := clientCtx.InterfaceRegistry().UnpackAny(txres.Tx, &tx); err != nil {
				log.Warn("GRPCGetTransactionByHash failed", "txHash", txHash, "err", err)
				return nil, errors.WithStack(err)
			}
			if tx == nil {
				return nil, fmt.Errorf("unpack tx error")
			}
			var txMemo string
			if tx.Body != nil {
				txMemo = tx.Body.Memo
			}
			return &GetTxResponse{
				Tx: &Tx{
					Body: TxBody{
						Memo: txMemo,
					},
				},
				TxResponse: &TxResponse{
					Height: fmt.Sprintf("%v", txres.Height),
					TxHash: txres.TxHash,
					Code:   txres.Code,
					Logs:   txres.Logs,
				},
			}, nil
		}
	}
	if err != nil {
		log.Warn("GRPCGetTransactionByHash failed", "txHash", txHash, "err", err)
	}
	return nil, wrapRPCQueryError(err, "GRPCGetTransactionByHash", txHash)
}

func (b *Bridge) GRPCGetBaseAccount(address string) (res *QueryAccountResponse, err error) {
	var ret authtypes.AccountI
	for _, rpcClient := range rpcClients {
		clientCtx := b.ClientContext.WithClient(rpcClient)
		ret, err = grpc.GetAccountInfo(ctx, clientCtx, address)
		if err == nil {
			return &QueryAccountResponse{
				Account: &BaseAccount{
					Address:       ret.GetAddress().String(),
					AccountNumber: fmt.Sprintf("%v", ret.GetAccountNumber()),
					Sequence:      fmt.Sprintf("%v", ret.GetSequence()),
				},
			}, nil
		}
	}
	if err != nil {
		log.Warn("GRPCGetBaseAccount failed", "address", address, "err", err)
	}
	return nil, wrapRPCQueryError(err, "GRPCGetBaseAccount", address)
}

func (b *Bridge) GRPCGetDenomBalance(address, denom string) (res sdk.Int, err error) {
	for _, rpcClient := range rpcClients {
		clientCtx := b.ClientContext.WithClient(rpcClient)
		res, err = grpc.GetDenomBalance(ctx, clientCtx, address, denom)
		if err == nil {
			return res, nil
		}
	}
	if err != nil {
		log.Warn("GRPCGetDenomBalance failed", "address", address, "denom", denom, "err", err)
	}
	return sdk.ZeroInt(), wrapRPCQueryError(err, "GRPCGetDenomBalance", address, denom)
}

func (b *Bridge) GRPCSimulateTx(simulateReq *SimulateRequest) (res *sdktx.SimulateResponse, err error) {
	for _, rpcClient := range rpcClients {
		clientCtx := b.ClientContext.WithClient(rpcClient)
		res, err = grpc.SimulateTx(ctx, clientCtx, []byte(simulateReq.TxBytes))
		if err == nil {
			return res, nil
		}
	}
	if err != nil {
		log.Warn("GRPCSimulateTx failed", "err", err)
	}
	return nil, wrapRPCQueryError(err, "GRPCSimulateTx")
}

func (b *Bridge) GRPCBroadcastTx(req *BroadcastTxRequest) (res *sdk.TxResponse, err error) {
	for _, rpcClient := range rpcClients {
		clientCtx := b.ClientContext.
			WithClient(rpcClient).
			WithBroadcastMode(flags.BroadcastSync)
		res, err = grpc.BroadcastRawTx(ctx, clientCtx, []byte(req.TxBytes))
		if err == nil {
			return res, nil
		}
	}
	if err != nil {
		log.Warn("GRPCBroadcastTx failed", "err", err)
	}
	return nil, wrapRPCQueryError(err, "GRPCBroadcastTx")
}

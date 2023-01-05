package cosmos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (b *Bridge) GRPCGetLatestBlockNumber() (uint64, error) {
	return 0, nil
}

func (b *Bridge) GRPCGetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	return 0, nil
}

func (b *Bridge) GRPCGetChainID() (string, error) {
	return "", nil
}

func (b *Bridge) GRPCGetTransactionByHash(txHash string) (*GetTxResponse, error) {
	return nil, nil
}

func (b *Bridge) GRPCGetBaseAccount(address string) (*QueryAccountResponse, error) {
	return nil, nil
}

func (b *Bridge) GRPCGetDenomBalance(address, denom string) (sdk.Int, error) {
	return sdk.ZeroInt(), nil
}

func (b *Bridge) GRPCSimulateTx(simulateReq *SimulateRequest) (string, error) {
	return "", nil
}

func (b *Bridge) GRPCBroadcastTx(req *BroadcastTxRequest) (string, error) {
	return "", nil
}

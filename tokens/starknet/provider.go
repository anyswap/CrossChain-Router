package starknet

import (
	"context"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet/rpcv02"
	"github.com/dontpanicdao/caigo/gateway"
	ctypes "github.com/dontpanicdao/caigo/types"
	"github.com/ethereum/go-ethereum/rpc"
)

var ctx = context.Background()

const (
	GATEWAYIDENTIFIER = "starknet.io"
	LOCALHOST         = "localhost"

	LATEST  = "latest"
	GOERLI  = "SN_GOERLI"
	MAINNET = "SN_MAIN"
)

var chainIDs = []string{"mainnet", "testnet", "devnet"}

// Provider abstracts RPC and gateway providers' interface
type Provider interface {
	Nonce(accountAddress string) (uint64, error)
	BlockNumber() (uint64, error)
	ChainID() (string, error)
	Call(call ctypes.FunctionCall) ([]string, error)
	Invoke(interface{}) (*ctypes.AddInvokeTransactionOutput, error)
	EstimateFee(call interface{}) (*ctypes.FeeEstimate, error)
	TransactionByHash(hash string) (rpcv02.Transaction, error)
	TransactionReceipt(hash string) (rpcv02.TransactionReceipt, error)
	WaitForTransaction(transactionHash ctypes.Hash, pollInterval time.Duration) (ctypes.TransactionState, error)
}

func NewProvider(url, stubChainID string) (Provider, error) {
	var chainID string
	for _, cid := range chainIDs {
		if strings.EqualFold(stubChainID, GetStubChainID(cid).String()) {
			chainID = cid
			break
		}
	}
	if chainID == "" {
		return nil, tokens.ErrNoBridgeForChainID
	}

	if strings.Contains(url, GATEWAYIDENTIFIER) || strings.Contains(url, LOCALHOST) {
		return NewGatewayProvider(url, chainID)
	}
	c, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, err
	}
	return &RPCProvider{rpcv02.NewProvider(c)}, nil
}

func NewGatewayProvider(url, chainID string) (Provider, error) {
	if strings.Contains(url, GATEWAYIDENTIFIER) || strings.Contains(url, LOCALHOST) {
		optBaseURL := gateway.WithBaseURL(url)
		optChainID := gateway.WithChain(chainID)
		return &GWProvider{gateway.NewProvider(optBaseURL, optChainID), url}, nil
	}
	c, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, err
	}
	return &RPCProvider{rpcv02.NewProvider(c)}, nil
}

// RPCProvider connects to Starknet full nodes
type RPCProvider struct {
	r *rpcv02.Provider
}

// GWProvider connects to Starknet official gateways
type GWProvider struct {
	gw  *gateway.GatewayProvider
	url string
}

func (p *RPCProvider) Nonce(contractAddress string) (uint64, error) {
	n, err := p.r.Nonce(ctx, rpcv02.BlockID{Tag: "latest"}, ctypes.HexToHash(contractAddress))
	if err != nil {
		return 0, err
	}
	parseInt, err := strconv.Atoi(*n)
	if err != nil {
		return 0, err
	}
	return uint64(parseInt), nil
}

func (p *RPCProvider) BlockNumber() (uint64, error) {
	return p.r.BlockNumber(ctx)
}

func (p *RPCProvider) ChainID() (string, error) {
	return p.r.ChainID(ctx)
}

func (p *RPCProvider) Call(call ctypes.FunctionCall) ([]string, error) {
	return p.r.Call(ctx, call, rpcv02.WithBlockTag(LATEST))
}

func (p *RPCProvider) Invoke(signedTx interface{}) (*ctypes.AddInvokeTransactionOutput, error) {
	tx, ok := signedTx.(rpcv02.BroadcastedInvokeV1Transaction)
	if !ok {
		return nil, tokens.ErrWrongRawTx
	}
	return p.r.AddInvokeTransaction(ctx, tx)
}

func (p *RPCProvider) EstimateFee(call interface{}) (*ctypes.FeeEstimate, error) {
	return p.r.EstimateFee(ctx, call, rpcv02.WithBlockTag("Latest"))
}

func (p *RPCProvider) TransactionByHash(hash string) (rpcv02.Transaction, error) {
	return p.r.TransactionByHash(ctx, ctypes.HexToHash(hash))
}

func (p *RPCProvider) TransactionReceipt(txHash string) (rpcv02.TransactionReceipt, error) {
	return p.r.TransactionReceipt(ctx, ctypes.HexToHash(txHash))
}

func (p *RPCProvider) WaitForTransaction(transactionHash ctypes.Hash, pollInterval time.Duration) (ctypes.TransactionState, error) {
	return p.r.WaitForTransaction(ctx, transactionHash, pollInterval)
}

func (p *GWProvider) Nonce(contractAddress string) (uint64, error) {
	n, err := p.gw.Nonce(ctx, contractAddress, "")
	if n.IsUint64() {
		return n.Uint64(), err
	}
	return 0, tokens.ErrUint64Nonce
}

func (p *GWProvider) BlockNumber() (uint64, error) {
	// Ref. `curl --location 'https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=latest'`
	b, err := p.gw.Block(ctx, &gateway.BlockOptions{Tag: LATEST})
	if err != nil {
		return 0, err
	}
	if b.BlockNumber == 0 {
		return 0, tokens.ErrLatestBlockNum
	}
	return uint64(b.BlockNumber), nil
}

func (p *GWProvider) ChainID() (string, error) {
	return p.gw.ChainID(ctx)
}

func (p *GWProvider) Call(call ctypes.FunctionCall) ([]string, error) {
	return p.gw.Call(ctx, call, "")
}

func (p *GWProvider) Invoke(signedTx interface{}) (*ctypes.AddInvokeTransactionOutput, error) {
	tx, ok := signedTx.(rpcv02.BroadcastedInvokeV1Transaction)
	if !ok {
		return nil, tokens.ErrWrongRawTx
	}
	var sigBN []*big.Int
	for _, s := range tx.Signature {
		sigBN = append(sigBN, ctypes.HexToBN(s))
	}

	body := ctypes.FunctionInvoke{
		MaxFee:       tx.MaxFee,
		Version:      ctypes.StrToBig(string(tx.Version)),
		Signature:    sigBN,
		Nonce:        tx.Nonce,
		Type:         TxTypeInvoke,
		FunctionCall: ctypes.FunctionCall{Calldata: tx.Calldata},
	}

	return p.gw.Invoke(ctx, body)
}

func (p *GWProvider) EstimateFee(call interface{}) (*ctypes.FeeEstimate, error) {
	invoke, ok := call.(ctypes.FunctionInvoke)
	if !ok {
		return nil, tokens.ErrInvalidInvokeInput
	}
	return p.gw.EstimateFee(ctx, invoke, "")
}

func (p *GWProvider) TransactionByHash(hash string) (rpcv02.Transaction, error) {
	tx, err := p.gw.TransactionByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	baseTx := rpcv02.CommonTransaction{
		TransactionHash: ctypes.HexToHash(tx.TransactionHash),
		Type:            rpcv02.TransactionType(tx.Type),
		MaxFee:          tx.MaxFee,
		Version:         ctypes.NumAsHex(tx.Version),
		Signature:       tx.Signature,
		Nonce:           tx.Nonce,
	}

	switch tx.Version {
	case "0x0":
		return rpcv02.InvokeTxnV0{
			CommonTransaction:  baseTx,
			ContractAddress:    ctypes.HexToHash(tx.ContractAddress),
			EntryPointSelector: tx.EntryPointSelector,
			Calldata:           tx.Calldata,
		}, nil
	default:
		return rpcv02.InvokeTxnV1{
			CommonTransaction: baseTx,
			SenderAddress:     ctypes.HexToHash(tx.SenderAddress),
			Calldata:          tx.Calldata,
		}, nil
	}
}

func (p *GWProvider) TransactionReceipt(txHash string) (rpcv02.TransactionReceipt, error) {
	receipt, err := p.gw.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, err
	}

	var events []rpcv02.Event
	for _, e := range receipt.Events {
		events = append(events, e.(rpcv02.Event))
	}
	msgSent := []rpcv02.MsgToL1{
		{ToAddress: receipt.L1ToL2ConsumedMessage.ToAddress, Payload: receipt.L1ToL2ConsumedMessage.Payload},
	}

	return rpcv02.CommonTransactionReceipt{
		TransactionHash: ctypes.HexToHash(receipt.TransactionHash),
		Status:          receipt.Status,
		BlockHash:       ctypes.HexToHash(receipt.BlockHash),
		BlockNumber:     uint64(receipt.BlockNumber),
		MessagesSent:    msgSent,
		Events:          events,
	}, nil
}

func (p *GWProvider) WaitForTransaction(transactionHash ctypes.Hash, pollInterval time.Duration) (ctypes.TransactionState, error) {
	_, receipt, err := p.gw.WaitForTransaction(ctx, transactionHash.String(), int(pollInterval), 10)
	if err != nil {
		return "", err
	}
	return receipt.Status, nil
}

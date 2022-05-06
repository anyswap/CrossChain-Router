package tron

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	tronaddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	tronclient "github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"google.golang.org/grpc"

	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/types"
	"github.com/anyswap/CrossChain-Router/v3/common"
)

var GRPC_TIMEOUT = time.Second * 15

func (b *Bridge) getClients() []*tronclient.GrpcClient {
	endpoints := b.GatewayConfig.APIAddress
	clis := make([]*client.GrpcClient, 0)
	for _, endpoint := range endpoints {
		cli := tronclient.NewGrpcClientWithTimeout(endpoint, GRPC_TIMEOUT)
		if cli != nil {
			clis = append(clis, cli)
		}
	}
	return clis
}

type RPCError struct {
	errs   []error
	method string
}

func (e *RPCError) log(msg error) {
	log.Warn("[Tron RPC error]", "method", e.method, "msg", msg)
	if len(e.errs) < 1 {
		e.errs = make([]error, 1)
	}
	e.errs = append(e.errs, msg)
}

func (e *RPCError) Error() error {
	return fmt.Errorf("[Tron RPC error] method: %v errors:%+v", e.method, e.errs)
}

// GetLatestBlockNumberOf call eth_blockNumber
func (b *Bridge) GetLatestBlockNumberOf(url string) (latest uint64, err error) {
	return 0, nil
}

// GetLatestBlockNumber returns current finalized block height
func (b *Bridge) GetLatestBlockNumber() (height uint64, err error) {
	rpcError := &RPCError{[]error{}, "GetLatestBlockNumber"}
	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		if err != nil {
			rpcError.log(err)
			continue
		}
		res, err := cli.GetNowBlock()
		if err == nil {
			if res.BlockHeader.RawData.Number > 0 {
				height = uint64(res.BlockHeader.RawData.Number)
				cli.Stop()
				break
			}
		} else {
			rpcError.log(err)
		}
		cli.Stop()
	}
	if height > 0 {
		return height, nil
	}
	return 0, rpcError.Error()
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	rpcError := &RPCError{[]error{}, "GetTransaction"}
	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		if err != nil {
			rpcError.log(err)
			continue
		}
		tx, err = cli.GetTransactionByID(txHash)
		if err == nil {
			cli.Stop()
			break
		}
		cli.Stop()
	}
	if err != nil {
		return nil, rpcError.Error()
	}
	return
}

// GetTransactionLog
func (b *Bridge) GetTransactionLog(txHash string) ([]*types.RPCLog, error) {
	rpcError := &RPCError{[]error{}, "GetTransactionLog"}
	var tx *core.TransactionInfo
	var err error
	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		if err != nil {
			rpcError.log(err)
			continue
		}
		tx, err = cli.GetTransactionInfoByID(txHash)
		if err == nil {
			cli.Stop()
			break
		}
		cli.Stop()
	}
	if err != nil {
		return nil, rpcError.Error()
	}
	tronlogs := tx.GetLog()
	logs := make([]*types.RPCLog,0)
	for _, tlog := range tronlogs {
		addr := common.BytesToAddress(tlog.Address)
		data := hexutil.Bytes(tlog.Data)
		ethlog := &types.RPCLog{
			Address: &addr,
			Topics: []common.Hash{},
			Data: &data,
			Removed: new(bool),
		}
		for _, topic := range tlog.Topics {
			ethlog.Topics = append(ethlog.Topics, common.BytesToHash(topic))
		}
		logs = append(logs, ethlog)
	}
	return logs, nil
}

// BroadcastTx broadcast tx to network
func (b *Bridge) BroadcastTx(tx *core.Transaction) (err error) {
	rpcError := &RPCError{[]error{}, "BroadcastTx"}
	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		if err != nil {
			rpcError.log(err)
			continue
		}
		res, err := cli.Broadcast(tx)
		if err == nil {
			cli.Stop()
			if res.Code != 0 {
				rpcError.log(fmt.Errorf("bad transaction: %v", string(res.GetMessage())))
			}
			return nil
		}
		rpcError.log(err)
	}
	return rpcError.Error()
}

// ChainID call eth_chainId
// Notice: eth_chainId return 0x0 for mainnet which is wrong (use net_version instead)
func (b *Bridge) ChainID() (*big.Int, error) {
	return b.TronChainID, nil
}

// NetworkID call net_version
func (b *Bridge) NetworkID() (*big.Int, error) {
	return nil, nil
}

// GetCode returns contract bytecode
func (b *Bridge) GetCode(contractAddress string) (data []byte, err error) {
	contractDesc := tronaddress.HexToAddress(anyToEth(contractAddress))
	if contractDesc.String() == "" {
		return nil, errors.New("invalid contract address")
	}
	message := new(api.BytesMessage)
	message.Value = contractDesc
	rpcError := &RPCError{[]error{}, "GetCode"}
	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		ctx, cancel := context.WithTimeout(context.Background(), GRPC_TIMEOUT)
		if err != nil {
			rpcError.log(err)
			cancel()
			continue
		}
		sm, err1 := cli.Client.GetContract(ctx, message)
		err = err1
		if err == nil {
			data = sm.Bytecode
			cli.Stop()
			cancel()
			break
		}
		cli.Stop()
		cancel()
	}
	if err != nil {
		return nil, rpcError.Error()
	}
	return data, nil
}

// GetBalance gets TRON token balance
func (b *Bridge) GetBalance(account string) (balance *big.Int, err error) {
	rpcError := &RPCError{[]error{}, "GetBalance"}
	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		if err != nil {
			rpcError.log(err)
			continue
		}
		res, err := cli.GetAccount(account)
		if err == nil {
			if res.Balance > 0 {
				balance = big.NewInt(int64(res.Balance))
				cli.Stop()
				break
			}
		} else {
			rpcError.log(err)
		}
		cli.Stop()
	}
	if balance.Cmp(big.NewInt(0)) > 0 {
		return balance, nil
	}
	return big.NewInt(0), rpcError.Error()
}

var SwapinFeeLimit int64 = 300000000 // 300 TRX
var ExtraExpiration int64 = 900000 // 15 min

func (b *Bridge) BuildTriggerConstantContractTx(from, contract string, dataBytes []byte) (tx *core.Transaction, err error) {
	fromAddr := tronaddress.HexToAddress(anyToEth(from))
	if fromAddr.String() == "" {
		return nil, errors.New("invalid from address")
	}
	contractAddr := tronaddress.HexToAddress(anyToEth(contract))
	if contractAddr.String() == "" {
		return nil, errors.New("invalid contract address")
	}
	ct := &core.TriggerSmartContract{
		OwnerAddress:    fromAddr.Bytes(),
		ContractAddress: contractAddr.Bytes(),
		Data:            dataBytes,
	}

	rpcError := &RPCError{[]error{}, "BuildTriggerConstantContractTx"}

	ctx, cancel := context.WithTimeout(context.Background(), GRPC_TIMEOUT)
	defer cancel()

	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		if err != nil {
			rpcError.log(err)
			continue
		}
		txext, err1 := cli.Client.TriggerConstantContract(ctx, ct)
		if txext != nil {
			txext.Transaction.RawData.FeeLimit = SwapinFeeLimit
			txext.Transaction.RawData.Expiration = txext.Transaction.RawData.Expiration + ExtraExpiration
		}
		err = err1
		if err == nil {
			tx = txext.Transaction
			cli.Stop()
			break
		}
		rpcError.log(err)
		cli.Stop()
	}
	if err != nil {
		return nil, rpcError.Error()
	}
	return tx, nil
}

// CallContract
func (b *Bridge) CallContract(contract string, data hexutil.Bytes, blockNumber string) (string, error) {
	contractAddr := tronaddress.HexToAddress(anyToEth(contract))
	if contractAddr.String() == "" {
		return "", errors.New("invalid contract address")
	}

	ctx, cancel := context.WithTimeout(context.Background(), GRPC_TIMEOUT)
	defer cancel()

	fromDesc := tronaddress.HexToAddress("410000000000000000000000000000000000000000")

	ct := &core.TriggerSmartContract{
		OwnerAddress:    fromDesc.Bytes(),
		ContractAddress: contractAddr.Bytes(),
		Data:            []byte(data),
	}

	rpcError := &RPCError{[]error{}, "CallContract"}
	var err error
	for _, cli := range b.getClients() {
		err = cli.Start(grpc.WithInsecure())
		if err != nil {
			rpcError.log(err)
			continue
		}
		txext, err1 := cli.Client.TriggerConstantContract(ctx, ct)
		if err1 != nil {
			continue
		}
		res := common.ToHex(txext.GetConstantResult()[0])
		return res, nil
	}
	return "", nil
}
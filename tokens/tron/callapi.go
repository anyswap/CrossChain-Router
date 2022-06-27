package tron

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	tronaddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	ethclient "github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/types"
	"google.golang.org/protobuf/proto"
)

var GRPC_TIMEOUT = time.Second * 15

/*func (b *Bridge) getClients() []*tronclient.GrpcClient {
	endpoints := b.GatewayConfig.APIAddress
	clis := make([]*client.GrpcClient, 0)
	for _, endpoint := range endpoints {
		cli := tronclient.NewGrpcClientWithTimeout(endpoint, GRPC_TIMEOUT)
		if cli != nil {
			clis = append(clis, cli)
		}
	}
	return clis
}*/

func post(url string, data string) ([]byte, error) {
	client := &http.Client{}
	var dataReader = strings.NewReader(data)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, dataReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bodyText, nil
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
	rpcError := &RPCError{[]error{}, "GetLatestBlockNumber"}
	apiurl := strings.TrimSuffix(url, "/") + `/wallet/getblockbylatestnum`
	res, err := post(apiurl, `{"num":1}`)
	if err != nil {
		rpcError.log(err)
		return 0, rpcError.Error()
	}
	var blocks map[string]interface{}
	err = json.Unmarshal(res, &blocks)
	if err != nil {
		rpcError.log(err)
		return 0, rpcError.Error()
	}
	if blocks["block"] == nil {
		rpcError.log(errors.New("parse error"))
		return 0, rpcError.Error()
	}
	blockLs, ok := blocks["block"].([]interface{})
	if !ok || len(blockLs) < 1 {
		rpcError.log(errors.New("parse error"))
		return 0, rpcError.Error()
	}
	block, ok := blockLs[0].(map[string]interface{})
	if !ok {
		rpcError.log(errors.New("parse error"))
		return 0, rpcError.Error()
	}
	header, ok := block["block_header"].(map[string]interface{})
	if !ok {
		rpcError.log(errors.New("parse error"))
		return 0, rpcError.Error()
	}
	raw, ok := header["raw_data"].(map[string]interface{})
	if !ok {
		rpcError.log(errors.New("parse error"))
		return 0, rpcError.Error()
	}
	numberF, ok := raw["number"].(float64)
	if !ok {
		rpcError.log(errors.New("parse error"))
		return 0, rpcError.Error()
	}
	return uint64(numberF), nil
}

// GetLatestBlockNumber returns current finalized block height
func (b *Bridge) GetLatestBlockNumber() (height uint64, err error) {
	rpcError := &RPCError{[]error{}, "GetLatestBlockNumber"}
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/getblockbylatestnum`
		res, err := post(apiurl, `{"num":1}`)
		if err != nil {
			rpcError.log(err)
			continue
		}
		var blocks map[string]interface{}
		err = json.Unmarshal(res, &blocks)
		if err != nil {
			rpcError.log(err)
			continue
		}
		if blocks["block"] == nil {
			rpcError.log(errors.New("parse error"))
			continue
		}
		blockLs, ok := blocks["block"].([]interface{})
		if !ok || len(blockLs) < 1 {
			rpcError.log(errors.New("parse error"))
			continue
		}
		block, ok := blockLs[0].(map[string]interface{})
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		header, ok := block["block_header"].(map[string]interface{})
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		raw, ok := header["raw_data"].(map[string]interface{})
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		numberF, ok := raw["number"].(float64)
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		return uint64(numberF), nil
	}
	if height > 0 {
		return height, nil
	}
	return 0, rpcError.Error()
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	rpcError := &RPCError{[]error{}, "GetTransaction"}
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/gettransactionbyid`
		res, err := post(apiurl, `{"value":"`+txHash+`"}`)
		if err != nil {
			rpcError.log(err)
			continue
		}
		tx, err := func(res []byte) (tx *core.Transaction, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("%v", r)
				}
			}()
			txi := make(map[string]interface{})
			tx = &core.Transaction{}
			err = json.Unmarshal(res, &txi)
			if err != nil {
				panic(err)
			}
			for _, reti := range txi["ret"].([]interface{}) {
				ret := reti.(map[string]interface{})["ret"]
				cret := reti.(map[string]interface{})["contractRet"]
				switch {
				case ret != nil:
					tx.Ret = append(tx.Ret, &core.Transaction_Result{Ret: core.Transaction_ResultCode(core.Transaction_ResultCode_value[ret.(string)])})
				case cret != nil:
					tx.Ret = append(tx.Ret, &core.Transaction_Result{ContractRet: core.Transaction_ResultContractResult(core.Transaction_ResultContractResult_value[cret.(string)])})
				default:
				}
			}
			for _, sig := range txi["signature"].([]interface{}) {
				bz, err1 := hex.DecodeString(sig.(string))
				if err1 != nil {
					panic(err1)
				}
				tx.Signature = append(tx.Signature, bz)
			}
			bz, _ := hex.DecodeString(txi["raw_data_hex"].(string))
			rawdata := &core.TransactionRaw{}
			err = proto.Unmarshal(bz, rawdata)
			if err != nil {
				panic(err)
			}
			tx.RawData = rawdata
			return tx, err
		}(res)
		if err != nil {
			rpcError.log(err)
			continue
		}
		return tx, nil
	}
	return
}

// GetTransactionLog
func (b *Bridge) GetTransactionLog(txHash string) ([]*types.RPCLog, error) {
	rpcError := &RPCError{[]error{}, "GetTransactionLog"}
	defer func() {
		if r := recover(); r != nil {
			rpcError.log(fmt.Errorf("%v", r))
		}
	}()
	var tronlogs []interface{}
	var err error
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/walletsolidity/gettransactioninfobyid`
		res, err1 := post(apiurl, `{"value":"`+txHash+`"}`)
		if err1 != nil {
			panic(err1)
		}
		txinfo := make(map[string]interface{})
		err = json.Unmarshal(res, &txinfo)
		if err != nil {
			rpcError.log(err)
			continue
		}
		var ok bool
		tronlogs, ok = txinfo["log"].([]interface{})
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
	}
	if err != nil {
		return nil, rpcError.Error()
	}
	logs := make([]*types.RPCLog, 0)
	for _, tlog := range tronlogs {
		tl, ok := tlog.(map[string]interface{})
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		addr := common.HexToAddress(tl["address"].(string))
		data, _ := hex.DecodeString(tl["data"].(string))
		hexdata := hexutil.Bytes(data)
		ethlog := &types.RPCLog{
			Address: &addr,
			Topics:  []common.Hash{},
			Data:    &hexdata,
			Removed: new(bool),
		}
		topics, _ := tl["topics"].([]interface{})
		if topics == nil && len(topics) == 0 {
			return nil, rpcError.Error()
		}
		for _, topic := range topics {
			ethlog.Topics = append(ethlog.Topics, common.HexToHash(topic.(string)))
		}
		logs = append(logs, ethlog)
	}
	return logs, nil
}

// BroadcastTx broadcast tx to network
func (b *Bridge) BroadcastTx(tx *core.Transaction) (err error) {
	rpcError := &RPCError{[]error{}, "BroadcastTx"}
	txhex := hex.EncodeToString(tx.GetRawData().GetData())
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/broadcasthex`
		res, err := post(apiurl, `{"transaction":"`+txhex+`"}`)
		if err != nil {
			rpcError.log(err)
			continue
		}
		result := make(map[string]interface{})
		err = json.Unmarshal(res, &result)
		if err != nil {
			rpcError.log(err)
			continue
		}
		success, ok := result["result"].(bool)
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		if success {
			return nil
		}
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
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/getcontract`
		res, err := post(apiurl, `{"value":"`+contractAddress+`","visible":false}`)
		if err != nil {
			rpcError.log(err)
			continue
		}
		result := make(map[string]interface{})
		err = json.Unmarshal(res, &result)
		if err != nil {
			rpcError.log(err)
			continue
		}
		//bytecode
		datastr, ok := result["bytecode"].(string)
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		data, err := hex.DecodeString(datastr)
		if err != nil {
			rpcError.log(errors.New("parse error"))
			continue
		}
		return data, nil
	}
	return nil, rpcError.Error()
}

// GetBalance gets TRON token balance
func (b *Bridge) GetBalance(account string) (balance *big.Int, err error) {
	rpcError := &RPCError{[]error{}, "GetBalance"}
	account = strings.TrimPrefix(account, "0x")
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/getaccount`
		res, err := post(apiurl, `{"value":"`+account+`","visible":false}`)
		if err != nil {
			rpcError.log(err)
			continue
		}
		result := make(map[string]interface{})
		err = json.Unmarshal(res, &result)
		if err != nil {
			rpcError.log(errors.New("parse error"))
			continue
		}
		bal, ok := result["balance"].(int64)
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		balance.SetUint64(uint64(bal))
		return balance, nil
	}
	if balance.Cmp(big.NewInt(0)) > 0 {
		return balance, nil
	}
	return big.NewInt(0), rpcError.Error()
}

func (b *Bridge) BuildTriggerConstantContractTx(from, contract string, selector string, paramater string, fee_limit int32) (tx *core.Transaction, err error) {
	fromAddr := tronaddress.HexToAddress(anyToEth(from))
	if fromAddr.String() == "" {
		return nil, errors.New("invalid from address")
	}
	contractAddr := tronaddress.HexToAddress(anyToEth(contract))
	if contractAddr.String() == "" {
		return nil, errors.New("invalid contract address")
	}

	rpcError := &RPCError{[]error{}, "BuildTriggerConstantContractTx"}

	tx = &core.Transaction{}
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/triggersmartcontract`
		res, err := post(apiurl, `{"owner_address":"`+fromAddr.Hex()+`","contract_address":"`+contractAddr.Hex()+`","function_selector":"`+selector+`","paramater":"`+paramater+`","fee_limit":"`+fmt.Sprint(fee_limit)+`"}`)
		if err != nil {
			rpcError.log(err)
			continue
		}
		result := make(map[string]interface{})
		err = json.Unmarshal(res, &result)
		if err != nil {
			rpcError.log(errors.New("parse error"))
			continue
		}
		raw, ok := result["raw_data_hex"].(string)
		if !ok {
			rpcError.log(errors.New("parse error"))
			continue
		}
		bz, err := hex.DecodeString(raw)
		if err != nil {
			panic(err)
		}
		rawdata := &core.TransactionRaw{}
		err = proto.Unmarshal(bz, rawdata)
		if err != nil {
			panic(err)
		}
		tx.RawData = rawdata
		return tx, nil
	}
	return nil, rpcError.Error()
}

// CallContract
func (b *Bridge) CallContract(contract string, data hexutil.Bytes, blockNumber string) (string, error) {
	contractAddr := tronaddress.HexToAddress(anyToEth(contract))
	if contractAddr.String() == "" {
		return "", errors.New("invalid contract address")
	}

	reqArgs := map[string]interface{}{
		"to":   anyToEth(contractAddr),
		"data": data,
	}
	gateway := b.GatewayConfig
	var result string
	var err error
	for _, apiAddress := range gateway.EVMAPIAddress {
		url := apiAddress
		err = ethclient.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_call", reqArgs, "latest")
		if err != nil && router.IsIniting {
			for i := 0; i < router.RetryRPCCountInInit; i++ {
				if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_call", reqArgs, "latest"); err == nil {
					return result, nil
				}
				time.Sleep(router.RetryRPCIntervalInInit)
			}
		}
		if err == nil {
			return result, nil
		}
	}
	if err != nil {
		logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
		logFunc("call CallContract failed", "contract", contract, "data", data, "err", err)
	}
	return "", err
}

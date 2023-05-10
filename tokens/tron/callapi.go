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
	if len(e.errs) == 0 {
		e.errs = make([]error, 0, 1)
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
	for _, endpoint := range b.GatewayConfig.AllGatewayURLs {
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

type rpcGetTxRes struct {
	Ret        []map[string]interface{} `json:"ret"`
	Signature  []string                 `json:"signature"`
	TxID       string                   `json:"txID"`
	RawData    map[string]interface{}   `json:"raw_data"`
	RawDataHex string                   `json:"raw_data_hex"`
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTronTransaction(txHash)
}

// GetTronTransaction get tx
func (b *Bridge) GetTronTransaction(txHash string) (tx *core.Transaction, err error) {
	rpcError := &RPCError{[]error{}, "GetTransaction"}
	for _, endpoint := range b.GatewayConfig.AllGatewayURLs {
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
			var txi rpcGetTxRes
			err = json.Unmarshal(res, &txi)
			if err != nil {
				return nil, err
			}
			tx = &core.Transaction{}
			for _, reti := range txi.Ret {
				cret := reti["contractRet"]
				if cret != nil {
					tx.Ret = append(tx.Ret, &core.Transaction_Result{ContractRet: core.Transaction_ResultContractResult(core.Transaction_ResultContractResult_value[cret.(string)])})
				}
			}
			for _, sig := range txi.Signature {
				bz, err1 := hex.DecodeString(sig)
				if err1 != nil {
					return nil, err1
				}
				tx.Signature = append(tx.Signature, bz)
			}
			bz, err := hex.DecodeString(txi.RawDataHex)
			if err != nil {
				return nil, err
			}
			rawdata := &core.TransactionRaw{}
			err = proto.Unmarshal(bz, rawdata)
			if err != nil {
				return nil, err
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

type rpcGetTxInfoRes struct {
	TxID            string                   `json:"id"`
	BlockNumber     uint64                   `json:"blockNumber"`
	BlockTimeStamp  uint64                   `json:"blockTimeStamp"`
	ContractResult  []string                 `json:"contractResult"`
	ContractAddress string                   `json:"contract_address"`
	Receipt         map[string]interface{}   `json:"receipt"`
	Result          string                   `json:"result,omitempty"`
	ResultMsg       string                   `json:"resMessage,omitempty"`
	Log             rpcLogSlice              `json:"log"`
	InternalTxs     []map[string]interface{} `json:"internal_transactions"`
}

type rpcLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
	Removed bool     `json:"removed"`
}

type rpcLogSlice []*rpcLog

func (logs rpcLogSlice) toRPCLog() []*types.RPCLog {
	res := make([]*types.RPCLog, len(logs))
	for i, l := range logs {
		address := common.HexToAddress(l.Address)
		data, _ := hex.DecodeString(l.Data)
		hexdata := hexutil.Bytes(data)
		topics := make([]common.Hash, len(l.Topics))
		for j, t := range l.Topics {
			topics[j] = common.HexToHash(t)
		}
		res[i] = &types.RPCLog{
			Address: &address,
			Topics:  topics,
			Data:    &hexdata,
			Removed: &l.Removed,
		}
	}
	return res
}

// IsStatusOk is status ok
func (r *rpcGetTxInfoRes) IsStatusOk() bool {
	stat, ok := r.Receipt["result"].(string)
	return ok && stat == "SUCCESS" && r.Result != "FAILED"
}

func (b *Bridge) GetTransactionInfo(txHash string) (*rpcGetTxInfoRes, error) {
	rpcError := &RPCError{[]error{}, "GetTransactionInfo"}
	defer func() {
		if r := recover(); r != nil {
			rpcError.log(fmt.Errorf("%v", r))
		}
	}()
	for _, endpoint := range b.GatewayConfig.AllGatewayURLs {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/walletsolidity/gettransactioninfobyid`
		res, err := post(apiurl, `{"value":"`+txHash+`"}`)
		if err != nil {
			rpcError.log(fmt.Errorf("rpc post error: %w", err))
			continue
		}
		var txInfo rpcGetTxInfoRes
		err = json.Unmarshal(res, &txInfo)
		if err != nil {
			rpcError.log(fmt.Errorf("json unmarshal error: %w", err))
			continue
		}
		return &txInfo, nil
	}
	return nil, errors.New("tx log not found")
}

// GetTransactionLog
func (b *Bridge) GetTransactionLog(txHash string) ([]*types.RPCLog, error) {
	txInfo, err := b.GetTransactionInfo(txHash)
	if err != nil {
		return nil, err
	}
	status, ok := txInfo.Receipt["result"].(string)
	if !ok || status != "SUCCESS" || txInfo.Result == "FAILED" {
		return nil, errors.New("tx status is not success")
	}
	return txInfo.Log.toRPCLog(), nil
}

// BroadcastTx broadcast tx to network
func (b *Bridge) BroadcastTx(tx *core.Transaction) (err error) {
	rpcError := &RPCError{[]error{}, "BroadcastTx"}
	protoData, err := proto.Marshal(tx)
	if err != nil {
		return err
	}
	txhex := fmt.Sprintf("%X", protoData)
	for _, endpoint := range b.GatewayConfig.AllGatewayURLs {
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
		if ok && success {
			return nil
		}
		rpcError.log(errors.New("parse error"))
		log.Error("BroadcastTx failed", "result", result, "txhex", txhex)
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
	rpcError := &RPCError{[]error{}, "GetCode"}
	for _, endpoint := range b.GatewayConfig.AllGatewayURLs {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/getcontract`
		res, err := post(apiurl, `{"value":"`+tronToEthWithPrefix(contractAddress)+`","visible":false}`)
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
	for _, endpoint := range b.GatewayConfig.AllGatewayURLs {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/getaccount`
		res, err := post(apiurl, `{"value":"`+tronToEthWithPrefix(account)+`","visible":false}`)
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

type rpcTx struct {
	RawData    map[string]interface{} `json:"raw_data"`
	RawDataHex string                 `json:"raw_data_hex"`
	TxID       string                 `json:"txID"`
	Visible    bool                   `json:"visible"`
}

type rpcTxResult struct {
	Result map[string]interface{} `json:"result,omitempty"`
	Tx     rpcTx                  `json:"transaction,omitempty"`
}

func (b *Bridge) BuildTriggerConstantContractTx(from, contract string, selector string, parameter string, fee_limit int64) (tx *core.Transaction, err error) {
	rpcError := &RPCError{[]error{}, "BuildTriggerConstantContractTx"}

	tx = &core.Transaction{}
	txdata := `{"owner_address":"` + tronToEthWithPrefix(from) + `","contract_address":"` + tronToEthWithPrefix(contract) + `","function_selector":"` + selector + `","parameter":"` + parameter + `","fee_limit":"` + fmt.Sprint(fee_limit) + `"}`
	log.Trace("BuildTriggerConstantContractTx", "txdata", txdata)
	for _, endpoint := range b.GatewayConfig.AllGatewayURLs {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/wallet/triggersmartcontract`
		res, err := post(apiurl, txdata)
		if err != nil {
			rpcError.log(fmt.Errorf("post error: %w", err))
			continue
		}
		var result rpcTxResult
		err = json.Unmarshal(res, &result)
		if err != nil {
			log.Error("BuildTriggerConstantContractTx json unmarshal failed", "data", common.ToHex(res))
			rpcError.log(errors.New("parse error: json"))
			continue
		}
		raw := result.Tx.RawDataHex
		if raw == "" {
			log.Error("BuildTriggerConstantContractTx post failed", "result", result)
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
		log.Trace("BuildTriggerConstantContractTx post success", "result", result)
		return tx, nil
	}
	return nil, rpcError.Error()
}

// CallContract
func (b *Bridge) CallContract(contract string, data hexutil.Bytes, blockNumber string) (string, error) {
	ethAddress, err := tronToEth(contract)
	if err != nil {
		return "", err
	}
	reqArgs := map[string]interface{}{
		"to":   ethAddress,
		"data": data,
	}
	gateway := b.GatewayConfig
	var result string
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

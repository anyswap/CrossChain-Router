package reef

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/types"
	substratetypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"

	"golang.org/x/crypto/blake2b"
)

var (
	errEmptyURLs              = errors.New("empty URLs")
	errTxInOrphanBlock        = errors.New("tx is in orphan block")
	errTxHashMismatch         = errors.New("tx hash mismatch with rpc result")
	errTxBlockHashMismatch    = errors.New("tx block hash mismatch with rpc result")
	errTxReceiptMissBlockInfo = errors.New("tx receipt missing block info")

	wrapRPCQueryError = tokens.WrapRPCQueryError
)

func (b *Bridge) GetMetadata() *string {
	if b.mustUpdate() {
		err := b.getMetadata(metadata)
		if err != nil {
			log.Warn("Get metadata error")
		} else {
			lastUpdateTime = uint64(time.Now().Second())
		}
	}
	return metadata
}

func (b *Bridge) mustUpdate() bool {
	if *metadata == "" || uint64(time.Now().Second()) - lastUpdateTime > mustUpdateGap {
		return true
	}
	return false
}

func (b *Bridge) getMetadata(metadata *string) (error) {
	gateway := b.GatewayConfig
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err := client.RPCPostWithTimeout(b.RPCClientTimeout, metadata, url, "state_getMetadata")
		if err != nil {
			return wrapRPCQueryError(err, "state_getMetadata")
		}
	}
	return nil
}

// GetLatestBlockNumberOf call eth_blockNumber
func (b *Bridge) GetLatestBlockNumberOf(url string) (latest uint64, err error) {
	var blockHash string
	err = client.RPCPostWithTimeout(b.RPCClientTimeout, &blockHash, url, "chain_getFinalisedHead")
	if err != nil {
		return 0, wrapRPCQueryError(err, "chain_getFinalisedHead")
	}
	var result *BlockResult
	err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "chain_getBlock", blockHash)
	if err != nil {
		return 0, wrapRPCQueryError(err, "chain_getBlock")
	}
	return common.GetUint64FromStr(result.Number)
}

// GetLatestBlockNumber call eth_blockNumber
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	gateway := b.GatewayConfig
	return b.getMaxLatestBlockNumber(gateway.APIAddress)
}

func (b *Bridge) getMaxLatestBlockNumber(urls []string) (maxHeight uint64, err error) {
	if len(urls) == 0 {
		return 0, errEmptyURLs
	}
	for _, url := range urls {
		height, err := b.GetLatestBlockNumberOf(url)
		if err == nil {
			if height > maxHeight {
				maxHeight = height
			}
		}
	}
	if maxHeight > 0 {
		return maxHeight, nil
	}
	return 0, wrapRPCQueryError(err, "chain_getBlock")
}

// GetBlockByHash call eth_getBlockByHash
func (b *Bridge) GetBlockByHash(blockHash string) (*BlockResult, error) {
	gateway := b.GatewayConfig
	return b.getBlockByHash(blockHash, gateway.APIAddress)
}

func (b *Bridge) getBlockByHash(blockHash string, urls []string) (result *BlockResult, err error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "chain_getBlock", blockHash, false)
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "chain_getBlock", blockHash)
}

// GetBlockByNumber call eth_getBlockByNumber
func (b *Bridge) GetBlockByNumber(number *big.Int) (blk *BlockResult, err error) {
	gateway := b.GatewayConfig
	for _, url := range gateway.APIAddress {
		var hash string
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &hash, url, "chain_getBlockHash", number.Uint64(), false)
		if err == nil && hash != "" {
			blk, err = b.getBlockByHash(hash, gateway.APIAddress)
			return
		}
	}
	return nil, wrapRPCQueryError(err, "chain_getBlockHash", number)
}

func (b *Bridge) GetBlockHashByNumber(number *big.Int) (hash string, err error) {
	gateway := b.GatewayConfig
	for _, url := range gateway.APIAddress {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &hash, url, "chain_getBlockHash", number.Uint64(), false)
		if err == nil && hash != "" {
			return hash, nil
		}
	}
	return "", wrapRPCQueryError(err, "chain_getBlockHash", number)
}

// GetTransaction impl
// tx key is concat with block hash and extrinsic index
func (b *Bridge) GetTransaction(extKey string) (interface{}, error) {
	return b.GetTransactionByHash(extKey)
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetTransactionByHash(extKey string) (tx *ExtrinsicResult, err error) {
	gateway := b.GatewayConfig
	tx, err = b.getTransactionByHash(extKey, gateway.APIAddress)
	if err != nil && tokens.IsRPCQueryOrNotFoundError(err) && len(gateway.APIAddressExt) > 0 {
		tx, err = b.getTransactionByHash(extKey, gateway.APIAddressExt)
	}
	return tx, err
}

func (b *Bridge) getTransactionByHash(extKeyRaw string, urls []string) (result *ExtrinsicResult, err error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}

	var extKey = ExtKeyFromRaw(extKeyRaw)
	block, err := b.getBlockByHash(extKey.BlockHash, urls)
	if err != nil {
		return nil, err
	}
	var events string
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &events, url, "state_getStorage", b.StorageKey(extKey))
		if err == nil && events != "" {
			return b.MakeExtrinsicResult(block, extKey.ExtIdx, events)
		}
	}
	return nil, wrapRPCQueryError(err, "state_getStorage", extKeyRaw)
}

// GetPendingTransactions call eth_pendingTransactions
func (b *Bridge) GetPendingTransactions() (result []*types.RPCTransaction, err error) {
	return nil, errors.New("GetPendingTransactions not implemented")
}

// GetContractLogs get contract logs
func (b *Bridge) GetContractLogs(contractAddresses []common.Address, logTopics [][]common.Hash, blockHeight uint64) ([]*types.RPCLog, error) {
	return nil, errors.New("GetContractLogs not implemented")
}

// GetLogs call eth_getLogs
func (b *Bridge) GetLogs(filterQuery *types.FilterQuery) (result []*types.RPCLog, err error) {
	return nil, errors.New("GetLogs not implemented")
}

// GetPoolNonce call eth_getTransactionCount
func (b *Bridge) GetPoolNonce(ss58Account, height string) (uint64, error) {
	gateway := b.GatewayConfig
	return b.getMaxPoolNonce(ss58Account, height, gateway.APIAddress)
}

func (b *Bridge) getMaxPoolNonce(ss58Account string, height string, urls []string) (maxNonce uint64, err error) {
	if len(urls) == 0 {
		return 0, errEmptyURLs
	}
	var success bool
	var result hexutil.Uint64
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "system_accountNextIndex", ss58Account)
		if err == nil {
			success = true
			if uint64(result) > maxNonce {
				maxNonce = uint64(result)
			}
		}
	}
	if success {
		return maxNonce, nil
	}
	return 0, wrapRPCQueryError(err, "system_accountNextIndex", ss58Account, height)
}

// SendSignedTransaction call eth_sendRawTransaction
func (b *Bridge) SendSignedTransaction(ext *Extrinsic) (extHash, extKey string, err error) {
	bz, err := substratetypes.Encode(ext.Extrinsic)
	if err != nil {
		return "", "", err
	}
	extHash = fmt.Sprintf("%#x", blake2b.Sum256(bz))
	log.Info("call author_submitAndWatchExtrinsic start", "txHash", extHash)
	gateway := b.GatewayConfig
	urlCount := len(gateway.APIAddressExt) + len(gateway.APIAddress)
	ch := make(chan *sendTxResult, urlCount)
	wg := new(sync.WaitGroup)
	wg.Add(urlCount)
	go func() {
		wg.Wait()
		close(ch)
		log.Info("call author_submitAndWatchExtrinsic finished", "txHash", extHash)
	}()
	for _, url := range gateway.APIAddress {
		go b.sendRawTransaction(wg, ext, url, ch)
	}
	for _, url := range gateway.APIAddressExt {
		go b.sendRawTransaction(wg, ext, url, ch)
	}
	for i := 0; i < urlCount; i++ {
		res := <-ch
		extKey, err = res.extKey, res.err
		if err == nil && extKey != "" {
			return extHash, extKey, nil
		}
	}
	return extHash, "", wrapRPCQueryError(err, "author_submitAndWatchExtrinsic")
}

type sendTxResult struct {
	extKey string
	err    error
}

func (b *Bridge) sendRawTransaction(wg *sync.WaitGroup, ext *Extrinsic, url string, ch chan<- *sendTxResult) {
	defer wg.Done()
	api, err := gsrpc.NewSubstrateAPI(url)
	if err != nil {
		ch <- &sendTxResult{"", err}
	}

	var metadata substratetypes.Metadata
	raw :=b.GetMetadata()
	err = substratetypes.DecodeFromHex(*raw, &metadata)
	if err != nil {
		ch <- &sendTxResult{"", err}
	}

	sub, err := api.RPC.Author.SubmitAndWatchExtrinsic(*ext.Extrinsic)
	if err != nil {
		ch <- &sendTxResult{"", err}
	}
	defer sub.Unsubscribe()
	for {
		status := <-sub.Chan()
		log.Trace("call author_submitAndWatchExtrinsic", "transaction status:", fmt.Sprintf("%#v", status), "url", url)

		if status.IsInBlock {
			result := ExtKey{status.AsInBlock.Hex(), 0}.String()
			ch <- &sendTxResult{result, err}
			return
		}
		if status.IsInvalid || status.IsDropped {
			ch <- &sendTxResult{"", fmt.Errorf("extrinsic fail: %#v", status)}
			return
		}
	}
}

// ChainID call eth_chainId
// Notice: eth_chainId return 0x0 for mainnet which is wrong (use net_version instead)
func (b *Bridge) ChainID() (*big.Int, error) {
	return big.NewInt(13939), nil
}

// NetworkID call net_version
func (b *Bridge) NetworkID() (*big.Int, error) {
	return nil, nil
}

// GetCode call eth_getCode
func (b *Bridge) GetCode(contract string) (code []byte, err error) {
	return nil, errors.New("GetCode not implemented")
}

// CallContract call evm_call
func (b *Bridge) CallContract(contract string, data hexutil.Bytes, storageLimit uint64, blockNumber string) (string, error) {
	reqArgs := map[string]interface{}{
		"from": b.WatcherAccount,
		"to":   contract,
		"data": data,
		"storageLimit": storageLimit,
		"value": 0,
	}
	gateway := b.GatewayConfig
	var result string
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_call", reqArgs, blockNumber)
		if err != nil && router.IsIniting {
			for i := 0; i < router.RetryRPCCountInInit; i++ {
				if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_call", reqArgs, blockNumber); err == nil {
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
	return "", wrapRPCQueryError(err, "evm_call", contract)
}

// GetBalance call evm_call erc20.balanceOf
func (b *Bridge) GetBalance(account string) (*big.Int, error) {
	hexData := "0x70a08231000000000000000000000000" + strings.TrimPrefix(account, "0x")
	data := common.FromHex(hexData)
	res, err := b.CallContract("0x0000000000000000000000000000000001000000", data, 0, "latest")
	if err != nil {
		return nil, err
	}
	bal, _ := new(big.Int).SetString(strings.TrimPrefix(res, "0x"), 16)
	return bal, nil
}

// EstimateGas call evm_estimateGas
func (b *Bridge) EstimateGas(from, to string, value *big.Int, data []byte) (uint64, error) {
	reqArgs := map[string]interface{}{
		"from":  from,
		"to":    to,
		"value": (*hexutil.Big)(value),
		"data":  hexutil.Bytes(data),
	}
	gateway := b.GatewayConfig
	var result hexutil.Uint64
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_estimateGas", reqArgs)
		if err == nil {
			return uint64(result), nil
		}
	}
	log.Warn("[rpc] estimate gas failed", "from", from, "to", to, "value", value, "data", hexutil.Bytes(data), "err", err)
	return 0, wrapRPCQueryError(err, "evm_estimateGas")
}

// EstimateResources call evm_estimateResources
func (b *Bridge) EstimateResources(from, to string, value *big.Int, data []byte) (uint64, error) {
	reqArgs := map[string]interface{}{
		"from":  from,
		"to":    to,
		"value": (*hexutil.Big)(value),
		"data":  hexutil.Bytes(data),
	}
	gateway := b.GatewayConfig
	var result hexutil.Uint64
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "evm_estimateResources", reqArgs)
		if err == nil {
			return uint64(result), nil
		}
	}
	log.Warn("[rpc] estimate gas failed", "from", from, "to", to, "value", value, "data", hexutil.Bytes(data), "err", err)
	return 0, wrapRPCQueryError(err, "evm_estimateResources")
}

func (b *Bridge) GetRuntimeVersionLatest() (rv *substratetypes.RuntimeVersion, err error) {
	urls := b.GatewayConfig.APIAddress
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	for _, url := range urls {
		api, err := gsrpc.NewSubstrateAPI(url)
		if err != nil { continue }
		rv, err := api.RPC.State.GetRuntimeVersionLatest()
		if err != nil { continue }
		return rv, nil
	}
	return nil, wrapRPCQueryError(err, "state_getRuntimeVersion")
}
package eth

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/callapi"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

var (
	errEmptyURLs              = errors.New("empty URLs")
	errTxInOrphanBlock        = errors.New("tx is in orphan block")
	errTxHashMismatch         = errors.New("tx hash mismatch with rpc result")
	errTxBlockHashMismatch    = errors.New("tx block hash mismatch with rpc result")
	errTxReceiptMissBlockInfo = errors.New("tx receipt missing block info")

	wrapRPCQueryError = tokens.WrapRPCQueryError
)

// NeedsFinalizeAPIAddress need special finalize api
func (b *Bridge) NeedsFinalizeAPIAddress() bool {
	switch b.ChainConfig.ChainID {
	case "1030", "71":
		return true
	default:
		return false
	}
}

// GetBlockConfirmations some chain may override this method
func (b *Bridge) GetBlockConfirmations(receipt *types.RPCTxReceipt) (uint64, error) {
	if b.ChainConfig != nil {
		switch b.ChainConfig.ChainID {
		case "1285", // kusama moonriver
			"336": // kusama shiden
			return callapi.KsmGetBlockConfirmations(b, receipt)
		case "1030", // conlux mainnet
			"71": // conflux testnet
			return callapi.CfxGetBlockConfirmations(b, receipt)
		case "42161": // arbitrum L2
			return callapi.ArbGetBlockConfirmations(b, receipt)
		}
	}
	// common implementation
	latest, err := b.GetLatestBlockNumber()
	if err != nil {
		return 0, err
	}
	blockNumber := receipt.BlockNumber.ToInt().Uint64()
	if latest > blockNumber {
		return latest - blockNumber, nil
	}
	return 0, nil
}

// GetLatestBlockNumberOf call eth_blockNumber
func (b *Bridge) GetLatestBlockNumberOf(url string) (latest uint64, err error) {
	var result string
	err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_blockNumber")
	if err == nil {
		return common.GetUint64FromStr(result)
	}
	return 0, wrapRPCQueryError(err, "eth_blockNumber")
}

// GetLatestBlockNumber call eth_blockNumber
func (b *Bridge) GetLatestBlockNumber() (maxHeight uint64, err error) {
	gateway := b.GatewayConfig
	var height uint64
	for _, url := range gateway.APIAddress {
		height, err = b.EvmContractBridge.GetLatestBlockNumberOf(url)
		if height > maxHeight && err == nil {
			maxHeight = height
		}
	}
	if maxHeight > 0 {
		return maxHeight, nil
	}
	return 0, wrapRPCQueryError(err, "eth_blockNumber")
}

// GetBlockByNumber call eth_getBlockByNumber
func (b *Bridge) GetBlockByNumber(number *big.Int) (*types.RPCBlock, error) {
	blockNumber := types.ToBlockNumArg(number)
	var err error
	for _, url := range b.AllGatewayURLs {
		var result *types.RPCBlock
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getBlockByNumber", blockNumber, false)
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getBlockByNumber", number)
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (interface{}, error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetTransactionByHash(txHash string) (tx *types.RPCTransaction, err error) {
	gateway := b.GatewayConfig
	tx, err = b.getTransactionByHash(txHash, gateway.APIAddress)
	if err != nil && tokens.IsRPCQueryOrNotFoundError(err) && len(gateway.APIAddressExt) > 0 {
		tx, err = b.getTransactionByHash(txHash, gateway.APIAddressExt)
	}
	return tx, err
}

func (b *Bridge) getTransactionByHash(txHash string, urls []string) (result *types.RPCTransaction, err error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	for _, url := range urls {
		start := time.Now()
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getTransactionByHash", txHash)
		log.Info("call getTransactionByHash finished", "txhash", txHash, "url", url, "timespent", time.Since(start).String())
		if err == nil && result != nil {
			if !common.IsEqualIgnoreCase(result.Hash.Hex(), txHash) {
				return nil, errTxHashMismatch
			}
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getTransactionByHash", txHash)
}

// GetTransactionByBlockNumberAndIndex get tx by block number and tx index
func (b *Bridge) GetTransactionByBlockNumberAndIndex(blockNumber *big.Int, txIndex uint) (result *types.RPCTransaction, err error) {
	for _, url := range b.AllGatewayURLs {
		result, err = b.getTransactionByBlockNumberAndIndex(blockNumber, txIndex, url)
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getTransactionByBlockNumberAndIndex", blockNumber, txIndex)
}

func (b *Bridge) getTransactionByBlockNumberAndIndex(blockNumber *big.Int, txIndex uint, url string) (result *types.RPCTransaction, err error) {
	err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getTransactionByBlockNumberAndIndex", types.ToBlockNumArg(blockNumber), hexutil.Uint64(txIndex))
	if err == nil && result != nil {
		return result, nil
	}
	return nil, wrapRPCQueryError(err, "eth_getTransactionByBlockNumberAndIndex", blockNumber, txIndex)
}

// GetTransactionReceipt call eth_getTransactionReceipt
func (b *Bridge) GetTransactionReceipt(txHash string) (receipt *types.RPCTxReceipt, err error) {
	gateway := b.GatewayConfig
	receipt, err = b.getTransactionReceipt(txHash, gateway.APIAddress)
	if err != nil && tokens.IsRPCQueryOrNotFoundError(err) && len(gateway.APIAddressExt) > 0 {
		return b.getTransactionReceipt(txHash, gateway.APIAddressExt)
	}
	return receipt, err
}

func (b *Bridge) getTransactionReceipt(txHash string, urls []string) (result *types.RPCTxReceipt, err error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	for _, url := range urls {
		start := time.Now()
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getTransactionReceipt", txHash)
		log.Info("call getTransactionReceipt finished", "txhash", txHash, "url", url, "timespent", time.Since(start).String())
		if err == nil && result != nil {
			if result.BlockNumber == nil || result.BlockHash == nil || result.TxIndex == nil {
				return nil, errTxReceiptMissBlockInfo
			}
			if !common.IsEqualIgnoreCase(result.TxHash.Hex(), txHash) {
				return nil, errTxHashMismatch
			}
			if params.IsCheckTxBlockIndexEnabled(b.ChainConfig.ChainID) {
				start = time.Now()
				tx, errt := b.getTransactionByBlockNumberAndIndex(result.BlockNumber.ToInt(), uint(*result.TxIndex), url)
				log.Info("call getTransactionByBlockNumberAndIndex finished", "txhash", txHash, "block", result.BlockNumber, "index", result.TxIndex, "url", url, "timespent", time.Since(start).String())
				if errt != nil {
					return nil, errt
				}
				if !common.IsEqualIgnoreCase(tx.Hash.Hex(), txHash) {
					log.Error("check tx with block and index failed", "txHash", txHash, "tx.Hash", tx.Hash.Hex(), "blockNumber", result.BlockNumber, "txIndex", result.TxIndex, "url", url)
					return nil, errTxInOrphanBlock
				}
			}
			if params.IsCheckTxBlockHashEnabled(b.ChainConfig.ChainID) {
				start = time.Now()
				errt := b.checkTxBlockHash(result.BlockNumber.ToInt(), *result.BlockHash)
				log.Info("call checkTxBlockHash finished", "txhash", txHash, "block", result.BlockNumber, "url", url, "timespent", time.Since(start).String())
				if errt != nil {
					return nil, errt
				}
			}
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getTransactionReceipt", txHash)
}

func (b *Bridge) checkTxBlockHash(blockNumber *big.Int, blockHash common.Hash) error {
	block, err := b.GetBlockByNumber(blockNumber)
	if err != nil {
		log.Warn("get block by number failed", "number", blockNumber.String(), "err", err)
		return err
	}
	if *block.Hash != blockHash {
		log.Warn("tx block hash mismatch", "number", blockNumber.String(), "have", blockHash.String(), "want", block.Hash.String())
		return errTxBlockHashMismatch
	}
	return nil
}

// GetPoolNonce call eth_getTransactionCount
func (b *Bridge) GetPoolNonce(address, height string) (mdPoolNonce uint64, err error) {
	start := time.Now()
	allPoolNonces := make([]uint64, 0, 10)
	account := common.HexToAddress(address)
	for _, url := range b.AllGatewayURLs {
		var result hexutil.Uint64
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getTransactionCount", account, height)
		if err == nil {
			allPoolNonces = append(allPoolNonces, uint64(result))
			log.Info("call eth_getTransactionCount success", "chainID", b.ChainConfig.ChainID, "url", url, "account", account, "nonce", uint64(result))
		}
	}
	if len(allPoolNonces) == 0 {
		log.Warn("GetPoolNonce failed", "chainID", b.ChainConfig.ChainID, "account", account, "height", height, "timespent", time.Since(start).String(), "err", err)
		return 0, wrapRPCQueryError(err, "eth_getTransactionCount", account, height)
	}
	sort.Slice(allPoolNonces, func(i, j int) bool {
		return allPoolNonces[i] < allPoolNonces[j]
	})
	count := len(allPoolNonces)
	mdInd := (count - 1) / 2
	if count%2 != 0 {
		mdPoolNonce = allPoolNonces[mdInd]
	} else {
		mdPoolNonce = (allPoolNonces[mdInd] + allPoolNonces[mdInd+1]) / 2
	}
	log.Info("GetPoolNonce success", "chainID", b.ChainConfig.ChainID, "account", account, "urls", len(b.AllGatewayURLs), "validCount", count, "median", mdPoolNonce, "timespent", time.Since(start).String())
	return mdPoolNonce, nil
}

// SuggestPrice call eth_gasPrice
func (b *Bridge) SuggestPrice() (*big.Int, error) {
	start := time.Now()
	defer func() {
		log.Infof("call getGasPrice timespent %v", time.Since(start).String())
	}()
	gateway := b.GatewayConfig
	calcMethod := params.GetCalcGasPriceMethod(b.ChainConfig.ChainID)
	switch calcMethod {
	case "first":
		return b.getGasPriceFromURL(gateway.APIAddress[0])
	case "max":
		return b.getMaxGasPrice()
	default:
		return b.getMedianGasPrice()
	}
}

func (b *Bridge) getGasPriceFromURL(url string) (*big.Int, error) {
	logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
	var err error
	for i := 0; i < 3; i++ {
		var result hexutil.Big
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_gasPrice")
		if err == nil {
			gasPrice := result.ToInt()
			logFunc("call eth_gasPrice success", "chainID", b.ChainConfig.ChainID, "url", url, "gasPrice", gasPrice)
			return gasPrice, nil
		}
		logFunc("call eth_gasPrice failed", "chainID", b.ChainConfig.ChainID, "url", url, "err", err)
	}
	return nil, wrapRPCQueryError(err, "eth_gasPrice")
}

func (b *Bridge) getMaxGasPrice() (*big.Int, error) {
	logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)

	var maxGasPrice *big.Int
	var maxGasPriceURL string

	var err error
	for _, url := range b.AllGatewayURLs {
		var result hexutil.Big
		if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_gasPrice"); err != nil {
			logFunc("call eth_gasPrice failed", "chainID", b.ChainConfig.ChainID, "url", url, "err", err)
			continue
		}
		gasPrice := result.ToInt()
		logFunc("call eth_gasPrice success", "chainID", b.ChainConfig.ChainID, "url", url, "gasPrice", gasPrice)
		if maxGasPrice == nil || gasPrice.Cmp(maxGasPrice) > 0 {
			maxGasPrice = gasPrice
			maxGasPriceURL = url
		}
	}
	if maxGasPrice == nil {
		log.Warn("getMaxGasPrice failed", "err", err)
		return nil, wrapRPCQueryError(err, "eth_gasPrice")
	}
	logFunc("getMaxGasPrice success", "chainID", b.ChainConfig.ChainID, "url", maxGasPriceURL, "maxGasPrice", maxGasPrice)
	return maxGasPrice, nil
}

// get median gas price as the rpc result fluctuates too widely
func (b *Bridge) getMedianGasPrice() (mdGasPrice *big.Int, err error) {
	allGasPrices := make([]*big.Int, 0, 10)
	for _, url := range b.AllGatewayURLs {
		var result hexutil.Big
		if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_gasPrice"); err != nil {
			log.Info("call eth_gasPrice failed", "chainID", b.ChainConfig.ChainID, "url", url, "err", err)
			continue
		}
		gasPrice := result.ToInt()
		log.Info("call eth_gasPrice success", "chainID", b.ChainConfig.ChainID, "url", url, "gasPrice", gasPrice)
		allGasPrices = append(allGasPrices, gasPrice)
	}
	if len(allGasPrices) == 0 {
		log.Warn("getMedianGasPrice failed", "err", err)
		return nil, wrapRPCQueryError(err, "eth_gasPrice")
	}
	sort.Slice(allGasPrices, func(i, j int) bool {
		return allGasPrices[i].Cmp(allGasPrices[j]) < 0
	})
	count := len(allGasPrices)
	mdInd := (count - 1) / 2
	if count%2 != 0 {
		mdGasPrice = allGasPrices[mdInd]
	} else {
		mdGasPrice = new(big.Int).Add(allGasPrices[mdInd], allGasPrices[mdInd+1])
		mdGasPrice.Div(mdGasPrice, big.NewInt(2))
	}
	log.Info("getMedianGasPrice success", "chainID", b.ChainConfig.ChainID, "urls", len(b.AllGatewayURLs), "validCount", count, "median", mdGasPrice)
	return mdGasPrice, nil
}

// SendSignedTransaction call eth_sendRawTransaction
func (b *Bridge) SendSignedTransaction(tx *types.Transaction) (txHash string, err error) {
	data, err := tx.MarshalBinary()
	if err != nil {
		return "", err
	}
	log.Info("call eth_sendRawTransaction start", "txHash", tx.Hash().String())
	hexData := common.ToHex(data)
	urlCount := len(b.AllGatewayURLs)
	ch := make(chan *sendTxResult, urlCount)
	wg := new(sync.WaitGroup)
	wg.Add(urlCount)
	go func(hash string, count int, start time.Time) {
		defer func() {
			if err := recover(); err != nil {
				const size = 4096
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				log.Errorf("call eth_sendRawTransaction crashed: %v\n%s", err, buf)
			}
		}()
		wg.Wait()
		close(ch)
		log.Info("call eth_sendRawTransaction finished", "txHash", hash, "count", count, "timespent", time.Since(start).String())
	}(tx.Hash().String(), urlCount, time.Now())
	for _, url := range b.AllGatewayURLs {
		go b.sendRawTransaction(wg, hexData, url, ch)
	}
	for i := 0; i < urlCount; i++ {
		res := <-ch
		txHash, err = res.txHash, res.err
		if err == nil && txHash != "" {
			return txHash, nil
		}
	}
	return "", wrapRPCQueryError(err, "eth_sendRawTransaction")
}

type sendTxResult struct {
	txHash string
	err    error
}

func (b *Bridge) sendRawTransaction(wg *sync.WaitGroup, hexData, url string, ch chan<- *sendTxResult) {
	defer wg.Done()
	var result string
	err := client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_sendRawTransaction", hexData)
	if err != nil {
		log.Trace("call eth_sendRawTransaction failed", "txHash", result, "url", url, "err", err)
	} else {
		log.Trace("call eth_sendRawTransaction success", "txHash", result, "url", url)
	}
	ch <- &sendTxResult{result, err}
}

// ChainID call eth_chainId
// Notice: eth_chainId return 0x0 for mainnet which is wrong (use net_version instead)
func (b *Bridge) ChainID() (*big.Int, error) {
	var err error
	for _, url := range b.AllGatewayURLs {
		var result hexutil.Big
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_chainId")
		if err == nil {
			return result.ToInt(), nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_chainId")
}

// NetworkID call net_version
func (b *Bridge) NetworkID() (*big.Int, error) {
	var err error
	for _, url := range b.AllGatewayURLs {
		var result string
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "net_version")
		if err == nil {
			version := new(big.Int)
			if _, ok := version.SetString(result, 10); !ok {
				return nil, fmt.Errorf("invalid net_version result %q", result)
			}
			return version, nil
		}
	}
	return nil, wrapRPCQueryError(err, "net_version")
}

// GetCode call eth_getCode
func (b *Bridge) GetCode(contract string) (code []byte, err error) {
	for _, url := range b.AllGatewayURLs {
		start := time.Now()
		var result hexutil.Bytes
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getCode", contract, "latest")
		log.Info("call getCode finished", "contract", contract, "url", url, "timespent", time.Since(start).String())
		if err == nil {
			return []byte(result), nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getCode", contract)
}

// CallContract call eth_call
func (b *Bridge) CallContract(contract string, data hexutil.Bytes, blockNumber string) (string, error) {
	reqArgs := map[string]interface{}{
		"to":   contract,
		"data": data,
	}
	var err error
LOOP:
	for _, url := range b.AllGatewayURLs {
		start := time.Now()
		var result string
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_call", reqArgs, blockNumber)
		if err != nil && router.IsIniting {
			for i := 0; i < router.RetryRPCCountInInit; i++ {
				retryStart := time.Now()
				if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_call", reqArgs, blockNumber); err == nil {
					return result, nil
				}
				if strings.Contains(err.Error(), "revert") ||
					strings.Contains(err.Error(), "VM execution error") {
					break LOOP
				}
				log.Warn("retry call contract failed", "chainID", b.ChainConfig.ChainID, "contract", contract, "times", i+1, "timespent", time.Since(retryStart).String(), "err", err)
				time.Sleep(router.RetryRPCIntervalInInit)
			}
		}
		if err == nil {
			log.Info("call contract success", "chainID", b.ChainConfig.ChainID, "contract", contract, "data", data, "url", url, "timespent", time.Since(start).String())
			return result, nil
		}
		log.Info("call contract failed", "chainID", b.ChainConfig.ChainID, "contract", contract, "data", data, "timespent", time.Since(start).String(), "err", err)
	}
	return "", wrapRPCQueryError(err, "eth_call", contract)
}

// GetBalance call eth_getBalance
func (b *Bridge) GetBalance(account string) (*big.Int, error) {
	var err error
	for _, url := range b.AllGatewayURLs {
		start := time.Now()
		var result hexutil.Big
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getBalance", account, params.GetBalanceBlockNumberOpt)
		log.Info("call getBalance finished", "account", account, "url", url, "timespent", time.Since(start).String())
		if err == nil {
			return result.ToInt(), nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getBalance", account)
}

// SuggestGasTipCap call eth_maxPriorityFeePerGas
func (b *Bridge) SuggestGasTipCap() (mdGasTipCap *big.Int, err error) {
	allGasTipCaps := make([]*big.Int, 0, 10)
	for _, url := range b.AllGatewayURLs {
		var result hexutil.Big
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_maxPriorityFeePerGas")
		if err == nil {
			allGasTipCaps = append(allGasTipCaps, result.ToInt())
			log.Info("call eth_maxPriorityFeePerGas success", "chainID", b.ChainConfig.ChainID, "url", url, "gasTipCap", result)
		}
	}
	if len(allGasTipCaps) == 0 {
		log.Warn("call eth_maxPriorityFeePerGas failed", "err", err)
		return nil, wrapRPCQueryError(err, "eth_maxPriorityFeePerGas")
	}
	sort.Slice(allGasTipCaps, func(i, j int) bool {
		return allGasTipCaps[i].Cmp(allGasTipCaps[j]) < 0
	})
	count := len(allGasTipCaps)
	mdInd := (count - 1) / 2
	if count%2 != 0 {
		mdGasTipCap = allGasTipCaps[mdInd]
	} else {
		mdGasTipCap = new(big.Int).Add(allGasTipCaps[mdInd], allGasTipCaps[mdInd+1])
		mdGasTipCap.Div(mdGasTipCap, big.NewInt(2))
	}
	log.Info("getMedianGasTipCap success", "chainID", b.ChainConfig.ChainID, "urls", len(b.AllGatewayURLs), "validCount", count, "median", mdGasTipCap)
	return mdGasTipCap, nil
}

// FeeHistory call eth_feeHistory
func (b *Bridge) FeeHistory(blockCount int, rewardPercentiles []float64) (*types.FeeHistoryResult, error) {
	var err error
	for _, url := range b.AllGatewayURLs {
		var result types.FeeHistoryResult
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_feeHistory", blockCount, "latest", rewardPercentiles)
		if err == nil {
			return &result, nil
		}
	}
	log.Warn("get fee history failed", "blockCount", blockCount, "err", err)
	return nil, wrapRPCQueryError(err, "eth_feeHistory", blockCount)
}

// GetBaseFee get base fee
func (b *Bridge) GetBaseFee(blockCount int) (*big.Int, error) {
	if blockCount == 0 { // from lastest block header
		block, err := b.GetBlockByNumber(nil)
		if err != nil {
			return nil, err
		}
		return block.BaseFee.ToInt(), nil
	}
	// from fee history
	feeHistory, err := b.FeeHistory(blockCount, nil)
	if err != nil {
		return nil, err
	}
	length := len(feeHistory.BaseFee)
	if length > 0 {
		return feeHistory.BaseFee[length-1].ToInt(), nil
	}
	return nil, wrapRPCQueryError(err, "eth_feeHistory", blockCount)
}

// EstimateGas call eth_estimateGas
func (b *Bridge) EstimateGas(from, to string, value *big.Int, data []byte) (uint64, error) {
	reqArgs := map[string]interface{}{
		"from":  from,
		"to":    to,
		"value": (*hexutil.Big)(value),
		"data":  hexutil.Bytes(data),
	}
	var err error
	for _, url := range b.AllGatewayURLs {
		var result hexutil.Uint64
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_estimateGas", reqArgs)
		if err == nil {
			return uint64(result), nil
		}
	}
	log.Warn("[rpc] estimate gas failed", "from", from, "to", to, "value", value, "data", hexutil.Bytes(data), "err", err)
	return 0, wrapRPCQueryError(err, "eth_estimateGas")
}

// GetContractLogs get contract logs
func (b *Bridge) GetContractLogs(contractAddresses common.Address, logTopics []common.Hash, blockHeight uint64) ([]*types.RPCLog, error) {
	height := new(big.Int).SetUint64(blockHeight)

	filter := &types.FilterQuery{
		FromBlock: height,
		ToBlock:   height,
		Addresses: []common.Address{contractAddresses},
		Topics:    [][]common.Hash{logTopics},
	}
	return b.GetLogs(filter)
}

// GetLogs call eth_getLogs
func (b *Bridge) GetLogs(filterQuery *types.FilterQuery) (result []*types.RPCLog, err error) {
	args, err := types.ToFilterArg(filterQuery)
	if err != nil {
		return nil, err
	}
	for _, apiAddress := range b.AllGatewayURLs {
		url := apiAddress
		start := time.Now()
		err = client.RPCPost(&result, url, "eth_getLogs", args)
		log.Info("call getLogs finished", "args", args, "url", url, "timespent", time.Since(start).String())
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getLogs")
}

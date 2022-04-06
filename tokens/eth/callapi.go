package eth

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
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

// GetLatestBlockNumberOf call eth_blockNumber
func (b *Bridge) GetLatestBlockNumberOf(url string) (latest uint64, err error) {
	if b.ChainConfig != nil { // after init
		switch b.ChainConfig.ChainID {
		case "1285": // kusama ecosystem
			return callapi.KsmGetLatestBlockNumberOf(url, b.GatewayConfig, b.RPCClientTimeout)
		}
	}
	var result string
	err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_blockNumber")
	if err == nil {
		return common.GetUint64FromStr(result)
	}
	return 0, wrapRPCQueryError(err, "eth_blockNumber")
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
	var result string
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_blockNumber")
		if err == nil {
			height, _ := common.GetUint64FromStr(result)
			if height > maxHeight {
				maxHeight = height
			}
		}
	}
	if maxHeight > 0 {
		return maxHeight, nil
	}
	return 0, wrapRPCQueryError(err, "eth_blockNumber")
}

// GetBlockByHash call eth_getBlockByHash
func (b *Bridge) GetBlockByHash(blockHash string) (*types.RPCBlock, error) {
	gateway := b.GatewayConfig
	return b.getBlockByHash(blockHash, gateway.APIAddress)
}

func (b *Bridge) getBlockByHash(blockHash string, urls []string) (result *types.RPCBlock, err error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getBlockByHash", blockHash, false)
		if err == nil && result != nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getBlockByHash", blockHash)
}

// GetBlockByNumber call eth_getBlockByNumber
func (b *Bridge) GetBlockByNumber(number *big.Int) (*types.RPCBlock, error) {
	gateway := b.GatewayConfig
	var result *types.RPCBlock
	var err error
	blockNumber := types.ToBlockNumArg(number)
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
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
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getTransactionByHash", txHash)
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
	gateway := b.GatewayConfig
	for _, url := range gateway.APIAddress {
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

// GetPendingTransactions call eth_pendingTransactions
func (b *Bridge) GetPendingTransactions() (result []*types.RPCTransaction, err error) {
	gateway := b.GatewayConfig
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_pendingTransactions")
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_pendingTransactions")
}

// GetTransactionReceipt call eth_getTransactionReceipt
func (b *Bridge) GetTransactionReceipt(txHash string) (receipt *types.RPCTxReceipt, url string, err error) {
	gateway := b.GatewayConfig
	receipt, url, err = b.getTransactionReceipt(txHash, gateway.APIAddress)
	if err != nil && tokens.IsRPCQueryOrNotFoundError(err) && len(gateway.APIAddressExt) > 0 {
		return b.getTransactionReceipt(txHash, gateway.APIAddressExt)
	}
	return receipt, url, err
}

func (b *Bridge) getTransactionReceipt(txHash string, urls []string) (result *types.RPCTxReceipt, rpcURL string, err error) {
	if len(urls) == 0 {
		return nil, "", errEmptyURLs
	}
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getTransactionReceipt", txHash)
		if err == nil && result != nil {
			if result.BlockNumber == nil || result.BlockHash == nil || result.TxIndex == nil {
				return nil, "", errTxReceiptMissBlockInfo
			}
			if !common.IsEqualIgnoreCase(result.TxHash.Hex(), txHash) {
				return nil, "", errTxHashMismatch
			}
			if params.IsCheckTxBlockIndexEnabled(b.ChainConfig.ChainID) {
				tx, errt := b.getTransactionByBlockNumberAndIndex(result.BlockNumber.ToInt(), uint(*result.TxIndex), url)
				if errt != nil {
					return nil, "", errt
				}
				if !common.IsEqualIgnoreCase(tx.Hash.Hex(), txHash) {
					log.Error("check tx with block and index failed", "txHash", txHash, "tx.Hash", tx.Hash.Hex(), "blockNumber", result.BlockNumber, "txIndex", result.TxIndex, "url", url)
					return nil, "", errTxInOrphanBlock
				}
			}
			if params.IsCheckTxBlockHashEnabled(b.ChainConfig.ChainID) {
				if err = b.checkTxBlockHash(result.BlockNumber.ToInt(), *result.BlockHash); err != nil {
					return nil, "", err
				}
			}
			return result, url, nil
		}
	}
	return nil, "", wrapRPCQueryError(err, "eth_getTransactionReceipt", txHash)
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

// GetContractLogs get contract logs
func (b *Bridge) GetContractLogs(contractAddresses []common.Address, logTopics [][]common.Hash, blockHeight uint64) ([]*types.RPCLog, error) {
	height := new(big.Int).SetUint64(blockHeight)

	filter := &types.FilterQuery{
		FromBlock: height,
		ToBlock:   height,
		Addresses: contractAddresses,
		Topics:    logTopics,
	}
	return b.GetLogs(filter)
}

// GetLogs call eth_getLogs
func (b *Bridge) GetLogs(filterQuery *types.FilterQuery) (result []*types.RPCLog, err error) {
	args, err := types.ToFilterArg(filterQuery)
	if err != nil {
		return nil, err
	}
	gateway := b.GatewayConfig
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getLogs", args)
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getLogs")
}

// GetPoolNonce call eth_getTransactionCount
func (b *Bridge) GetPoolNonce(address, height string) (uint64, error) {
	account := common.HexToAddress(address)
	gateway := b.GatewayConfig
	return b.getMaxPoolNonce(account, height, gateway.APIAddress)
}

func (b *Bridge) getMaxPoolNonce(account common.Address, height string, urls []string) (maxNonce uint64, err error) {
	if len(urls) == 0 {
		return 0, errEmptyURLs
	}
	var success bool
	var result hexutil.Uint64
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getTransactionCount", account, height)
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
	return 0, wrapRPCQueryError(err, "eth_getTransactionCount", account, height)
}

// SuggestPrice call eth_gasPrice
func (b *Bridge) SuggestPrice() (*big.Int, error) {
	gateway := b.GatewayConfig
	calcMethod := params.GetCalcGasPriceMethod(b.ChainConfig.ChainID)
	switch calcMethod {
	case "first":
		return b.getGasPriceFromURL(gateway.APIAddress[0])
	case "max":
		return b.getMaxGasPrice(gateway.APIAddress, gateway.APIAddressExt)
	default:
		return b.getMedianGasPrice(gateway.APIAddress, gateway.APIAddressExt)
	}
}

func (b *Bridge) getGasPriceFromURL(url string) (*big.Int, error) {
	logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
	var result hexutil.Big
	var err error
	for i := 0; i < 3; i++ {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_gasPrice")
		if err == nil {
			gasPrice := result.ToInt()
			logFunc("getGasPriceFromURL success", "url", url, "gasPrice", gasPrice)
			return gasPrice, nil
		}
		logFunc("call eth_gasPrice failed", "url", url, "err", err)
	}
	return nil, wrapRPCQueryError(err, "eth_gasPrice")
}

func (b *Bridge) getMaxGasPrice(urlsSlice ...[]string) (*big.Int, error) {
	logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)

	var maxGasPrice *big.Int
	var maxGasPriceURL string

	var result hexutil.Big
	var err error
	for _, urls := range urlsSlice {
		for _, url := range urls {
			if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_gasPrice"); err != nil {
				logFunc("call eth_gasPrice failed", "url", url, "err", err)
				continue
			}
			gasPrice := result.ToInt()
			if maxGasPrice == nil || gasPrice.Cmp(maxGasPrice) > 0 {
				maxGasPrice = gasPrice
				maxGasPriceURL = url
			}
		}
	}
	if maxGasPrice == nil {
		log.Warn("getMaxGasPrice failed", "err", err)
		return nil, wrapRPCQueryError(err, "eth_gasPrice")
	}
	logFunc("getMaxGasPrice success", "url", maxGasPriceURL, "maxGasPrice", maxGasPrice)
	return maxGasPrice, nil
}

// get median gas price as the rpc result fluctuates too widely
func (b *Bridge) getMedianGasPrice(urlsSlice ...[]string) (*big.Int, error) {
	logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)

	allGasPrices := make([]*big.Int, 0, 10)
	urlCount := 0

	var result hexutil.Big
	var err error
	for _, urls := range urlsSlice {
		urlCount += len(urls)
		for _, url := range urls {
			if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_gasPrice"); err != nil {
				logFunc("call eth_gasPrice failed", "url", url, "err", err)
				continue
			}
			gasPrice := result.ToInt()
			allGasPrices = append(allGasPrices, gasPrice)
		}
	}
	if len(allGasPrices) == 0 {
		log.Warn("getMedianGasPrice failed", "err", err)
		return nil, wrapRPCQueryError(err, "eth_gasPrice")
	}
	sort.Slice(allGasPrices, func(i, j int) bool {
		return allGasPrices[i].Cmp(allGasPrices[j]) < 0
	})
	var mdGasPrice *big.Int
	count := len(allGasPrices)
	mdInd := (count - 1) / 2
	if count%2 != 0 {
		mdGasPrice = allGasPrices[mdInd]
	} else {
		mdGasPrice = new(big.Int).Add(allGasPrices[mdInd], allGasPrices[mdInd+1])
		mdGasPrice.Div(mdGasPrice, big.NewInt(2))
	}
	logFunc("getMedianGasPrice success", "urls", urlCount, "count", count, "median", mdGasPrice)
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
	gateway := b.GatewayConfig
	urlCount := len(gateway.APIAddressExt) + len(gateway.APIAddress)
	ch := make(chan *sendTxResult, urlCount)
	wg := new(sync.WaitGroup)
	wg.Add(urlCount)
	go func() {
		wg.Wait()
		close(ch)
		log.Info("call eth_sendRawTransaction finished", "txHash", txHash)
	}()
	for _, url := range gateway.APIAddress {
		go b.sendRawTransaction(wg, hexData, url, ch)
	}
	for _, url := range gateway.APIAddressExt {
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

func (b *Bridge) sendRawTransaction(wg *sync.WaitGroup, hexData string, url string, ch chan<- *sendTxResult) {
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
	gateway := b.GatewayConfig
	var result hexutil.Big
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_chainId")
		if err == nil {
			return result.ToInt(), nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_chainId")
}

// NetworkID call net_version
func (b *Bridge) NetworkID() (*big.Int, error) {
	gateway := b.GatewayConfig
	var result string
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
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
	gateway := b.GatewayConfig
	code, err = b.getCode(contract, gateway.APIAddress)
	if err != nil && len(gateway.APIAddressExt) > 0 {
		return b.getCode(contract, gateway.APIAddressExt)
	}
	return code, err
}

func (b *Bridge) getCode(contract string, urls []string) ([]byte, error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	var result hexutil.Bytes
	var err error
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getCode", contract, "latest")
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
	gateway := b.GatewayConfig
	var result string
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_call", reqArgs, blockNumber)
		if err != nil && router.IsIniting {
			for i := 0; i < router.RetryRPCCountInInit; i++ {
				if err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_call", reqArgs, blockNumber); err == nil {
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
	return "", wrapRPCQueryError(err, "eth_call", contract)
}

// GetBalance call eth_getBalance
func (b *Bridge) GetBalance(account string) (*big.Int, error) {
	gateway := b.GatewayConfig
	var result hexutil.Big
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_getBalance", account, params.GetBalanceBlockNumberOpt)
		if err == nil {
			return result.ToInt(), nil
		}
	}
	return nil, wrapRPCQueryError(err, "eth_getBalance", account)
}

// SuggestGasTipCap call eth_maxPriorityFeePerGas
func (b *Bridge) SuggestGasTipCap() (maxGasTipCap *big.Int, err error) {
	gateway := b.GatewayConfig
	if len(gateway.APIAddressExt) > 0 {
		maxGasTipCap, err = b.getMaxGasTipCap(gateway.APIAddressExt)
	}
	maxGasTipCap2, err2 := b.getMaxGasTipCap(gateway.APIAddress)
	if err2 == nil {
		if maxGasTipCap == nil || maxGasTipCap2.Cmp(maxGasTipCap) > 0 {
			maxGasTipCap = maxGasTipCap2
		}
	} else {
		err = err2
	}
	if maxGasTipCap != nil {
		return maxGasTipCap, nil
	}
	return nil, err
}

func (b *Bridge) getMaxGasTipCap(urls []string) (maxGasTipCap *big.Int, err error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	var success bool
	var result hexutil.Big
	for _, url := range urls {
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_maxPriorityFeePerGas")
		if err == nil {
			success = true
			if maxGasTipCap == nil || result.ToInt().Cmp(maxGasTipCap) > 0 {
				maxGasTipCap = result.ToInt()
			}
		}
	}
	if success {
		return maxGasTipCap, nil
	}
	return nil, wrapRPCQueryError(err, "eth_maxPriorityFeePerGas")
}

// FeeHistory call eth_feeHistory
func (b *Bridge) FeeHistory(blockCount int, rewardPercentiles []float64) (*types.FeeHistoryResult, error) {
	gateway := b.GatewayConfig
	result, err := b.getFeeHistory(gateway.APIAddress, blockCount, rewardPercentiles)
	if err != nil && len(gateway.APIAddressExt) > 0 {
		result, err = b.getFeeHistory(gateway.APIAddressExt, blockCount, rewardPercentiles)
	}
	return result, err
}

func (b *Bridge) getFeeHistory(urls []string, blockCount int, rewardPercentiles []float64) (*types.FeeHistoryResult, error) {
	if len(urls) == 0 {
		return nil, errEmptyURLs
	}
	var result types.FeeHistoryResult
	var err error
	for _, url := range urls {
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
	gateway := b.GatewayConfig
	var result hexutil.Uint64
	var err error
	for _, apiAddress := range gateway.APIAddress {
		url := apiAddress
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &result, url, "eth_estimateGas", reqArgs)
		if err == nil {
			return uint64(result), nil
		}
	}
	log.Warn("[rpc] estimate gas failed", "from", from, "to", to, "value", value, "data", hexutil.Bytes(data), "err", err)
	return 0, wrapRPCQueryError(err, "eth_estimateGas")
}

package solana

import (
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

var (
	// RPCCall alias of tokens.RPCCall
	RPCCall = tokens.RPCCall

	wrapRPCQueryError = tokens.WrapRPCQueryError
)

// GetLatestBlockNumberOf call getSlot
func (b *Bridge) GetLatestBlockNumberOf(url string) (uint64, error) {
	return getMaxLatestBlockNumber([]string{url})
}

// GetLatestBlockNumber call getSlot
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	gateway := b.GatewayConfig
	return getMaxLatestBlockNumber(gateway.APIAddress)
}

func getMaxLatestBlockNumber(urls []string) (maxHeight uint64, err error) {
	// use getSlot intead of getBlockHeight as the latter is incorrectly too small
	callMethod := "getSlot"
	obj := map[string]interface{}{
		"commitment": "finalized",
	}
	var result uint64
	for _, url := range urls {
		err = client.RPCPost(&result, url, callMethod, obj)
		if err == nil && result > maxHeight {
			maxHeight = result
		}
	}
	if maxHeight > 0 {
		return maxHeight, nil
	}
	return 0, wrapRPCQueryError(err, callMethod)
}

// GetLatestBlockhash get latest block hash
// This method is only available in solana-core v1.9 or newer.
// Please use getRecentBlockhash for solana-core v1.8
func (b *Bridge) GetLatestBlockhash() (result *types.GetLatestBlockhashResult, err error) {
	obj := map[string]string{
		"commitment": "confirmed",
	}
	callMethod := "getLatestBlockhash"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, obj)
	return result, err
}

// GetRecentBlockhash get recent block hash
func (b *Bridge) GetRecentBlockhash() (result *types.GetRecentBlockhashResult, err error) {
	obj := map[string]string{
		"commitment": "confirmed",
	}
	callMethod := "getRecentBlockhash"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, obj)
	return result, err
}

// GetFeeForMessage get fee for message
// This method is only available in solana-core v1.9 or newer.
// Please use getFees for solana-core v1.8
func (b *Bridge) GetFeeForMessage(blockhash, message string) (result uint64, err error) {
	obj := map[string]string{
		"commitment": "confirmed",
	}
	callMethod := "getFeeForMessage"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, blockhash, message, obj)
	return result, err
}

// GetFees get fees
func (b *Bridge) GetFees() (result *types.GetFeesResult, err error) {
	obj := map[string]string{
		"commitment": "confirmed",
	}
	callMethod := "getFees"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, obj)
	return result, err
}

// GetBlock get block
func (b *Bridge) GetBlock(slot uint64, fullTx bool) (result *types.GetBlockResult, err error) {
	transactionDetails := "full"
	if !fullTx {
		transactionDetails = "signatures"
	}
	obj := map[string]interface{}{
		"encoding":           "json",
		"commitment":         "confirmed",
		"transactionDetails": transactionDetails,
	}
	callMethod := "getBlock"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, slot, obj)
	return result, err
}

// GetAccountInfo get account info
func (b *Bridge) GetAccountInfo(account, encoding string) (result *types.GetAccountInfoResult, err error) {
	if encoding == "" {
		encoding = "base64"
	}
	obj := map[string]interface{}{
		"encoding":   encoding,
		"commitment": "finalized",
	}
	callMethod := "getAccountInfo"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, account, obj)
	if err == nil && result != nil && result.Value == nil {
		return nil, tokens.ErrNotFound
	}
	return result, err
}

// GetBalance get balance
func (b *Bridge) GetBalance(publicKey string) (result *types.GetBalanceResult, err error) {
	obj := map[string]string{
		"commitment": "confirmed",
	}
	callMethod := "getBalance"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, publicKey, obj)
	return result, err
}

// GetProgramAccounts get program accounts
func (b *Bridge) GetProgramAccounts(account, encoding string, filters []map[string]interface{}) (result types.GetProgramAccountsResult, err error) {
	if encoding == "" {
		encoding = "base64"
	}
	obj := map[string]interface{}{
		"encoding":   encoding,
		"commitment": "finalized",
	}
	if len(filters) != 0 {
		obj["filters"] = filters
	}
	callMethod := "getProgramAccounts"
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, account, obj)
	return result, err
}

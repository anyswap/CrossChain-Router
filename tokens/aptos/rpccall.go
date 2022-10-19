package aptos

import (
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var wrapRPCQueryError = tokens.WrapRPCQueryError

// GetLedger get ledger info
func (b *Bridge) GetLedger() (result *LedgerInfo, err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		result, err = cli.GetLedger()
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetLedger")
}

// GetAccount get account info
func (b *Bridge) GetAccount(address string) (result *AccountInfo, err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		result, err = cli.GetAccount(address)
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetAccount")
}

// GetAccountResource get account resource
func (b *Bridge) GetAccountResource(address, resourceType string) (result *CoinInfoResource, err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		result, err = cli.GetAccountResource(address, resourceType)
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetAccountResource")
}

// GetTransactions get tx by hash
func (b *Bridge) GetTransactions(txHash string) (result *TransactionInfo, err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		result, err = cli.GetTransactions(txHash)
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetTransactions")
}

// GetTransactionsNotPending get non pending tx by hash
func (b *Bridge) GetTransactionsNotPending(txHash string) (result *TransactionInfo, err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		result, err = cli.GetTransactionsNotPending(txHash)
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetTransactionsNotPending")
}

// EstimateGasPrice estimate gas price
func (b *Bridge) EstimateGasPrice() (result *GasEstimate, err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		result, err = cli.EstimateGasPrice()
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "EstimateGasPrice")
}

// GetSigningMessage get signing message
func (b *Bridge) GetSigningMessage(request interface{}) (result *string, err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		result, err = cli.GetSigningMessage(request)
		if err == nil {
			return result, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetSigningMessage")
}

// SimulateTranscation simulate tx
func (b *Bridge) SimulateTranscation(request interface{}, publikKey string) (err error) {
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		err = cli.SimulateTranscation(request, publikKey)
		if err == nil {
			return nil
		}
	}
	return wrapRPCQueryError(err, "SimulateTranscation")
}

// SubmitTranscation submit to all urls
func (b *Bridge) SubmitTranscation(tx *Transaction) (result *TransactionInfo, err error) {
	var success bool
	var temp *TransactionInfo
	cli := RestClient{Timeout: b.RPCClientTimeout}
	for _, url := range b.AllGatewayURLs {
		cli.Url = url
		temp, err = cli.SubmitTranscation(tx)
		if err == nil {
			result = temp
			success = true
		} else {
			log.Error("SubmitTranscation error", "err", err)
		}
	}
	if success {
		return result, nil
	}
	return nil, wrapRPCQueryError(err, "SubmitTranscation")
}

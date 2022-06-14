package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// GetLatestBlockNumber get latest block height
func GetLatestBlock(url string) (interface{}, error) {
	return nil, tokens.ErrNotImplemented
}

func sendTransaction(url string, signedTx interface{}) (string, error) {
	return "", tokens.ErrNotImplemented
}

// GetTransactionByHash get tx by hash
func GetTransactionByHash(url, txHash string) (*ElectTx, error) {
	var result ElectTx
	var err error
	restApi := url + "/tx/" + txHash
	err = client.RPCGet(&result, restApi)
	if err == nil {
		return &result, nil
	}
	return nil, err
}

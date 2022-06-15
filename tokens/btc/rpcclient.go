package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

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

func EstimateFeePerKb(url string, blocks int) (fee int64, err error) {
	var result map[int]float64
	restApi := url + "/fee-estimates"
	err = client.RPCGet(&result, restApi)
	if err != nil {
		return 0, err
	}
	return int64(result[blocks] * 1000), nil
}

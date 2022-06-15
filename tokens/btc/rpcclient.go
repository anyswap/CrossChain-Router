package btc

import (
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

// PostTransaction call post to /tx
func PostTransaction(url, txHex string) (txHash string, err error) {
	restApi := url + "/tx"
	txHash, err = client.RPCRawPost(restApi, txHex)
	if err == nil {
		return txHash, nil
	}
	return "", err
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

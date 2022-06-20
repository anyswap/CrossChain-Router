package btc

import (
	"sort"

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

func FindUtxos(url string, addr string) (result []*ElectUtxo, err error) {
	restApi := url + "/address/" + addr + "/utxo"
	err = client.RPCGet(&result, restApi)
	if err == nil {
		sort.Sort(SortableElectUtxoSlice(result))
		return result, nil
	}
	return nil, err
}

func GetLatestBlockNumber(url string) (result uint64, err error) {
	restApi := url + "/blocks/tip/height"
	err = client.RPCGet(&result, restApi)
	if err == nil {
		return result, nil
	}
	return 0, err
}

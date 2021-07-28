package eth

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

var (
	cachedPairsMap = make(map[string]string) // key is of format `chainid:token0:token1` (lowercase)
)

func getCachedPairKey(chainID, token0, token1 string) string {
	return strings.ToLower(fmt.Sprintf("%v:%v:%v", chainID, token0, token1))
}

// GetPairFor call "getPair(address,address)"
func (b *Bridge) GetPairFor(factory, token0, token1 string) (string, error) {
	// first search in cache
	key := getCachedPairKey(b.ChainConfig.ChainID, token0, token1)
	cachedPairs := cachedPairsMap[key]
	if cachedPairs != "" {
		return cachedPairs, nil
	}

	faunHash := common.FromHex("0xe6a43905")
	data := make([]byte, 68)
	copy(data[:4], faunHash)
	copy(data[4:36], common.HexToAddress(token0).Hash().Bytes())
	copy(data[36:68], common.HexToAddress(token1).Hash().Bytes())
	res, err := b.CallContract(factory, data, "latest")
	if err != nil {
		return "", err
	}
	pairs := common.BytesToAddress(common.GetData(common.FromHex(res), 0, 32)).LowerHex()
	cachedPairsMap[key] = pairs
	cachedPairsMap[getCachedPairKey(b.ChainConfig.ChainID, token1, token0)] = pairs
	return pairs, nil
}

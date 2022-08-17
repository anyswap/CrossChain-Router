package cardano

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func GetTransactionMetadata(url string, msgID [32]byte) (interface{}, error) {
	return nil, tokens.ErrNotImplemented
}

func GetTransactionByHash(url string, msgID [32]byte) (interface{}, error) {
	return nil, tokens.ErrNotImplemented
}

func GetLatestBlockNumber(url string) (uint64, error) {
	return 0, tokens.ErrNotImplemented
}

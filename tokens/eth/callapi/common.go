package callapi

import (
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	wrapRPCQueryError = tokens.WrapRPCQueryError
)

// EvmBridge evm bridge interface
// import and use eth.Bridge will cause import cycle
// so define a new interface here
type EvmBridge interface {
	tokens.IBridge

	CallContract(contract string, data hexutil.Bytes, blockNumber string) (string, error)
}

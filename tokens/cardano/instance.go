package cardano

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// BridgeInstance btc bridge instance
var BridgeInstance BridgeInterface

// BridgeInterface btc bridge interface
type BridgeInterface interface {
	tokens.IBridge

	QueryUtxoOnChain(address string) (map[UtxoKey]AssetsMap, error)
	BuildAggregateTx(swapId string, utxos map[UtxoKey]AssetsMap) (*RawTransaction, error)
	AggregateTx() (string, error)
}

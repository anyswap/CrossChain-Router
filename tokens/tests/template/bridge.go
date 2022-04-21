package template

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
)

// Bridge bridge impl struct
type Bridge struct {
	*tokens.CrossChainBridgeBase
}

// NewCrossChainBridge new bridge instance
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
}

// RegisterSwap register swap.
// used in `RegisterRouterSwap` server rpc.
func (b *Bridge) RegisterSwap(txHash string, args *tokens.RegisterArgs) ([]*tokens.SwapTxInfo, []error) {
	return nil, []error{tokens.ErrNotImplemented}
}

// VerifyTransaction verify swap tx is valid and success on chain with needed confirmations.
func (b *Bridge) VerifyTransaction(txHash string, ars *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	return nil, tokens.ErrNotImplemented
}

// BuildRawTransaction build tx with specified args.
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	return nil, tokens.ErrNotImplemented
}

// VerifyMsgHash verify message hash is same.
// 'message hash' here is the real content (usually a hash) which will be signed.
// used in `accept` work for oracles to replay the same tx on destination chain.
// oracle will only accept a sign info if and only if the oracle can
// verify the tx and rebuild a tx and ensure the message hash is same.
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHash []string) error {
	return tokens.ErrNotImplemented
}

// MPCSignTransaction mpc sign tx.
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	return nil, "", tokens.ErrNotImplemented
}

// SendTransaction send signed raw tx.
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	return "", tokens.ErrNotImplemented
}

// GetTransaction get tx by hash.
func (b *Bridge) GetTransaction(txHash string) (interface{}, error) {
	return nil, tokens.ErrNotImplemented
}

// GetTransactionStatus get tx status by hash.
// get tx related infos like block height, confirmations, receipts etc.
// These infos is used to verify tx is acceptable.
// you can extend `TxStatus` if fields in it is not enough to do the checking.
func (b *Bridge) GetTransactionStatus(txHash string) (*tokens.TxStatus, error) {
	return nil, tokens.ErrNotImplemented
}

// GetLatestBlockNumber get latest block number through gateway urls.
// used in `GetRouterSwap` server rpc.
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	return 0, tokens.ErrNotImplemented
}

// GetLatestBlockNumberOf get latest block number of specified url.
// used in `AdjustGatewayOrder` function.
func (b *Bridge) GetLatestBlockNumberOf(url string) (uint64, error) {
	return 0, tokens.ErrNotImplemented
}

// IsValidAddress check if given `address` is valid on this chain.
// prevent swap to an invalid `bind` address which will make assets loss.
func (b *Bridge) IsValidAddress(address string) bool {
	return false
}

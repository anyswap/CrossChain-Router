// Package eth test eth router by implementing `tokens.IBridge` interface.
// this testing implementation is using the product `token/eth` impl.
// this way implementation is just an example, in reality we need
// impl the `tokens.IBridge` interface from scratch, and take it into
// the product implementation after this testing is passed.
package eth

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	ethpro "github.com/anyswap/CrossChain-Router/v3/tokens/eth"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
)

// Bridge bridge impl struct
type Bridge struct {
	*ethpro.Bridge
}

// NewCrossChainBridge new bridge instance
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		Bridge: ethpro.NewCrossChainBridge(),
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	b.Signer = types.MakeSigner("EIP155", b.ChainConfig.GetChainID())
}

// RegisterSwap register swap.
// used in `RegisterRouterSwap` server rpc.
func (b *Bridge) RegisterSwap(txHash string, args *tokens.RegisterArgs) ([]*tokens.SwapTxInfo, []error) {
	return b.Bridge.RegisterSwap(txHash, args)
}

// VerifyTransaction verify swap tx is valid and success on chain with needed confirmations.
func (b *Bridge) VerifyTransaction(txHash string, ars *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	return b.Bridge.VerifyTransaction(txHash, ars)
}

// BuildRawTransaction build tx with specified args.
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	return b.Bridge.BuildRawTransaction(args)
}

// VerifyMsgHash verify message hash is same.
// 'message hash' here is the real content (usally a hash) which will be signed.
// used in `accept` work for oracles to replay the same tx on destination chain.
// oracle will only accept a sign info if and only if the oracle can
// verify the tx and rebuild a tx and ensure the message hash is same.
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHash []string) error {
	return b.Bridge.VerifyMsgHash(rawTx, msgHash)
}

// MPCSignTransaction mpc sign tx.
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	return b.Bridge.MPCSignTransaction(rawTx, args)
}

// SendTransaction send signed raw tx.
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	return b.Bridge.SendTransaction(signedTx)
}

// GetTransaction get tx by hash.
func (b *Bridge) GetTransaction(txHash string) (interface{}, error) {
	return b.Bridge.GetTransaction(txHash)
}

// GetTransactionStatus get tx status by hash.
// get tx related infos like block height, confirmations, receipts etc.
// These infos is used to verify tx is acceptable.
// you can extend `TxStatus` if fields in it is not enough to do the checking.
func (b *Bridge) GetTransactionStatus(txHash string) (*tokens.TxStatus, error) {
	return b.Bridge.GetTransactionStatus(txHash)
}

// GetLatestBlockNumber get latest block number through gateway urls.
// used in `GetRouterSwap` server rpc.
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	return b.Bridge.GetLatestBlockNumber()
}

// GetLatestBlockNumberOf get latest block number of specified url.
// used in `AdjustGatewayOrder` function.
func (b *Bridge) GetLatestBlockNumberOf(url string) (uint64, error) {
	return b.Bridge.GetLatestBlockNumberOf(url)
}

// IsValidAddress check if given `address` is valid on this chain.
// prevent swap to an invalid `bind` address which will make assets loss.
func (b *Bridge) IsValidAddress(address string) bool {
	return b.Bridge.IsValidAddress(address)
}

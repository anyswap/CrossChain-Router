package impl

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// BridgeConfig bridge config
type BridgeConfig struct {
	SupportNonce bool
}

// Bridge bridge
type Bridge struct {
	*BridgeConfig
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge(cfg *BridgeConfig) *Bridge {
	return &Bridge{
		BridgeConfig: cfg,
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
	return nil
}

// RegisterSwap register swap.
// used in `RegisterRouterSwap` server rpc.
func (b *Bridge) RegisterSwap(txHash string, args *tokens.RegisterArgs) ([]*tokens.SwapTxInfo, []error) {
	return nil, nil
}

// VerifyTransaction verify swap tx is valid and success on chain with needed confirmations.
func (b *Bridge) VerifyTransaction(txHash string, ars *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	return nil, nil
}

// BuildRawTransaction build tx with specified args.
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	return nil, nil
}

// VerifyMsgHash verify message hash is same.
// 'message hash' here is the real content (usually a hash) which will be signed.
// used in `accept` work for oracles to replay the same tx on destination chain.
// oracle will only accept a sign info if and only if the oracle can
// verify the tx and rebuild a tx and ensure the message hash is same.
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHash []string) error {
	return nil
}

// MPCSignTransaction mpc sign tx.
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	return nil, "", nil
}

// SendTransaction send signed raw tx.
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	return "", nil
}

// GetTransaction get tx by hash.
func (b *Bridge) GetTransaction(txHash string) (interface{}, error) {
	return nil, nil
}

// GetTransactionStatus get tx status by hash.
// get tx related infos like block height, confirmations, receipts etc.
// These infos is used to verify tx is acceptable.
// you can extend `TxStatus` if fields in it is not enough to do the checking.
func (b *Bridge) GetTransactionStatus(txHash string) (*tokens.TxStatus, error) {
	return nil, nil
}

// GetLatestBlockNumber get latest block number through gateway urls.
// used in `GetRouterSwap` server rpc.
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	return 0, nil
}

// GetLatestBlockNumberOf get latest block number of specified url.
// used in `AdjustGatewayOrder` function.
func (b *Bridge) GetLatestBlockNumberOf(url string) (uint64, error) {
	return 0, nil
}

// GetBalance get balance is used for checking budgets to prevent DOS attacking
func (b *Bridge) GetBalance(account string) (*big.Int, error) {
	return nil, nil
}

// IsValidAddress check if given `address` is valid on this chain.
// prevent swap to an invalid `bind` address which will make assets loss.
func (b *Bridge) IsValidAddress(address string) bool {
	return false
}

// PublicKeyToAddress public key to address
func (b *Bridge) PublicKeyToAddress(pubKey string) (string, error) {
	return "", nil
}

package iota

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
)

var (
	defaultFee       int64  = 10
	accountReserve          = big.NewInt(10000000)
	tfPartialPayment uint32 = 0x00020000
)

// BuildRawTransaction build raw tx
//nolint:funlen,gocyclo // ok
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	return
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, destTag *uint32, amount *big.Int, err error) {
	return
}

func getPaymentAmount(amount *big.Int, token *tokens.TokenConfig) (*data.Amount, error) {
	return nil, nil
}

func (b *Bridge) getMinReserveFee() *big.Int {
	config := params.GetRouterConfig()
	if config == nil {
		return big.NewInt(0)
	}
	minReserve := params.GetMinReserveFee(b.ChainConfig.ChainID)
	if minReserve == nil {
		minReserve = big.NewInt(100000) // default 0.1 XRP
	}
	return minReserve
}

func (b *Bridge) setExtraArgs(args *tokens.BuildTxArgs) (*tokens.AllExtras, error) {
	return nil, nil
}

// GetTxBlockInfo impl NonceSetter interface
func (b *Bridge) GetTxBlockInfo(txHash string) (blockHeight, blockTime uint64) {
	return
}

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	return 0, nil
}

// GetSeq returns account tx sequence
func (b *Bridge) GetSeq(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

	if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) { // increase automatically
		nonce = b.GetSwapNonce(args.From)
		return &nonce, nil
	}

	nonce, err = b.GetPoolNonce(args.From, "pending")
	if err != nil {
		return nil, err
	}
	nonce = b.AdjustNonce(args.From, nonce)
	return &nonce, nil
}

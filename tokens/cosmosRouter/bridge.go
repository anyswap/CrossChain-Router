package cosmosRouter

import (
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.NonceSetter
	_ tokens.NonceSetter = &Bridge{}
)

// Bridge near bridge
type Bridge struct {
	*base.NonceSetterBase
	*cosmosSDK.CosmosRestClient
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		NonceSetterBase:  base.NewNonceSetterBase(),
		CosmosRestClient: cosmosSDK.NewCosmosRestClient([]string{""}, "", ""),
	}
}

// GetLatestBlockNumber gets latest block number
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	return b.CosmosRestClient.GetLatestBlockNumber()
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionByHash get tx response by hash
func (b *Bridge) GetTransactionByHash(txHash string) (*cosmosSDK.GetTxResponse, error) {
	return b.CosmosRestClient.GetTransactionByHash(txHash)
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)
	if res, err := b.CosmosRestClient.GetTransactionByHash(txHash); err != nil {
		log.Trace(b.ChainConfig.BlockChain+" Bridge::GetElectTransactionStatus fail", "tx", txHash, "err", err)
		return status, err
	} else {
		if res.TxResponse.Code != 0 {
			return status, tokens.ErrTxWithWrongStatus
		}
		if txHeight, err := strconv.ParseUint(res.TxResponse.Height, 10, 64); err != nil {
			return status, err
		} else {
			status.BlockHeight = txHeight
		}
		if blockNumber, err := b.GetLatestBlockNumber(); err != nil {
			return status, err
		} else {
			if blockNumber > status.BlockHeight {
				status.Confirmations = blockNumber - status.BlockHeight
			}
			return status, nil
		}
	}
}

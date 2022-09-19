package cosmosHub

import (
	"math/big"
	"sync"

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

	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
)

// Bridge near bridge
type Bridge struct {
	*base.NonceSetterBase
	*cosmosSDK.CosmosRestClient
}

// SupportsChainID supports chainID
func SupportsChainID(chainID *big.Int) bool {
	supportedChainIDsInit.Do(func() {
		supportedChainIDs[GetStubChainID(mainnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(testnetNetWork).String()] = true
	})
	return supportedChainIDs[chainID.String()]
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		NonceSetterBase:  base.NewNonceSetterBase(),
		CosmosRestClient: cosmosSDK.NewCosmosRestClient([]string{""}),
	}
}

// GetStubChainID get stub chainID
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("CosmosHub"))
	switch network {
	case mainnetNetWork:
	case testnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(1))
	default:
		log.Fatalf("unknown network %v", network)
	}
	stubChainID.Mod(stubChainID, tokens.StubChainIDBase)
	stubChainID.Add(stubChainID, tokens.StubChainIDBase)
	return stubChainID
}

// GetLatestBlockNumber gets latest block number
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	return b.CosmosRestClient.GetLatestBlockNumber("")
}

// GetLatestBlockNumberOf gets latest block number from single api
func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	return b.CosmosRestClient.GetLatestBlockNumber(apiAddress)
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
	txStatus := &tokens.TxStatus{}
	if res, err := b.CosmosRestClient.GetTransactionByHash(txHash); err != nil {
		log.Trace(b.ChainConfig.BlockChain+" Bridge::GetElectTransactionStatus fail", "tx", txHash, "err", err)
		return status, err
	} else {

		if res.TxResponse.Code != 0 {
			return status, tokens.ErrTxWithWrongStatus
		}
		txStatus.BlockHeight = uint64(res.TxResponse.Height)
		if blockNumber, err := b.GetLatestBlockNumber(); err != nil {
			return status, err
		} else {
			if blockNumber > txStatus.BlockHeight {
				txStatus.Confirmations = blockNumber - txStatus.BlockHeight
			}
			return status, nil
		}
	}
}

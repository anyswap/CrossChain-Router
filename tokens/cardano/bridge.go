package cardano

import (
	"errors"
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
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
	devnetNetWork  = "devnet"
)

// Bridge block bridge inherit from btc bridge
type Bridge struct {
	*base.NonceSetterBase
	RPCClientTimeout int
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	instance := &Bridge{
		NonceSetterBase:  base.NewNonceSetterBase(),
		RPCClientTimeout: 60,
	}
	BridgeInstance = instance
	return instance
}

// SupportsChainID supports chainID
func SupportsChainID(chainID *big.Int) bool {
	supportedChainIDsInit.Do(func() {
		supportedChainIDs[GetStubChainID(mainnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(testnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(devnetNetWork).String()] = true
	})
	return supportedChainIDs[chainID.String()]
}

// GetStubChainID get stub chainID
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("CARDANO"))
	switch network {
	case mainnetNetWork:
	case testnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(1))
	case devnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(2))
	default:
		log.Fatalf("unknown network %v", network)
	}
	stubChainID.Mod(stubChainID, tokens.StubChainIDBase)
	stubChainID.Add(stubChainID, tokens.StubChainIDBase)
	return stubChainID
}

// GetLatestBlockNumber gets latest block number
func (b *Bridge) GetLatestBlockNumber() (num uint64, err error) {
	if blockNumber, err := GetLatestBlockNumber(); err == nil {
		return blockNumber, nil
	} else {
		return 0, err
	}
}

// GetLatestBlockNumberOf gets latest block number from single api
func (b *Bridge) GetLatestBlockNumberOf(url string) (num uint64, err error) {
	return GetLatestBlockNumber()
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetTransactionByHash(txHash string) (*Transaction, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetTransactionByHash(url, txHash)
		if err == nil {
			return result, nil
		}
	}
	return nil, tokens.ErrTxNotFound
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetUtxosByAddress(address string) (*[]Output, error) {
	if !b.IsValidAddress(address) {
		return nil, errors.New("GetUtxosByAddress address is empty")
	}
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetUtxosByAddress(url, address)
		if err == nil {
			return result, nil
		}
	}
	return nil, tokens.ErrOutputLength
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)
	if res, err := b.GetTransactionByHash(txHash); err != nil {
		return nil, err
	} else {
		if !res.ValidContract {
			return nil, tokens.ErrTxIsNotValidated
		} else {
			if lastHeight, err := b.GetLatestBlockNumber(); err != nil {
				return nil, err
			} else {
				status.Confirmations = lastHeight - res.Block.SlotNo
				status.BlockHeight = res.Block.SlotNo
				status.Receipt = nil
				return status, nil
			}
		}
	}
}

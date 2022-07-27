package iota

import (
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/websockets"
	iotago "github.com/iotaledger/iota.go/v2"
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
	return &Bridge{
		NonceSetterBase:  base.NewNonceSetterBase(),
		RPCClientTimeout: 60,
	}
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
	stubChainID := new(big.Int).SetBytes([]byte("IOTA"))
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
// For ripple, GetLatestBlockNumber returns current ledger version
func (b *Bridge) GetLatestBlockNumber() (num uint64, err error) {
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	for _, url := range urls {
		if blockNumber, err := GetLatestBlockNumber(url); err == nil {
			return blockNumber, nil
		} else {
			log.Error("GetLatestBlockNumber", "err", err)
		}
	}
	return 0, tokens.ErrGetNodeInfo
}

// GetLatestBlockNumberOf gets latest block number from single api
// For ripple, GetLatestBlockNumberOf returns current ledger version
func (b *Bridge) GetLatestBlockNumberOf(url string) (num uint64, err error) {
	if blockNumber, err := GetLatestBlockNumber(url); err == nil {
		return blockNumber, nil
	} else {
		return 0, err
	}
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetTransactionMetadata(txHash string) (txRes *iotago.MessageMetadataResponse, err error) {
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	if msgID, err := ConvertMessageID(txHash); err != nil {
		return nil, err
	} else {
		for _, url := range urls {
			if metadataResponse, err := GetTransactionMetadata(url, msgID); err == nil {
				return metadataResponse, nil
			} else {
				log.Error("GetTransactionMetadata", "err", err)
			}
		}
		return nil, tokens.ErrTxNotFound
	}
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetTransactionByHash(txHash string) (txRes *iotago.Message, err error) {
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	if msgID, err := ConvertMessageID(txHash); err != nil {
		return nil, err
	} else {
		for _, url := range urls {
			if messageResponse, err := GetTransactionByHash(url, msgID); err == nil {
				return messageResponse, nil
			} else {
				log.Error("GetTransactionByHash", "err", err)
			}
		}
		return nil, tokens.ErrTxNotFound
	}
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)

	if tx, err := b.GetTransactionMetadata(txHash); err != nil {
		return nil, err
	} else {
		status.Receipt = nil
		inledger := tx.ReferencedByMilestoneIndex
		status.BlockHeight = uint64(*inledger)

		if latest, err := b.GetLatestBlockNumber(); err == nil && latest > uint64(*inledger) {
			status.Confirmations = latest - uint64(*inledger)
		}
	}

	return status, nil
}

// GetBalance gets balance
func (b *Bridge) GetBalance(accountAddress string) (*big.Int, error) {
	return nil, nil
}

// GetAccount returns account
func (b *Bridge) GetAccount(address string) (acctRes *websockets.AccountInfoResult, err error) {
	return
}

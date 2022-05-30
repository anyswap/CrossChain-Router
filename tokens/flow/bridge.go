package flow

import (
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	sdk "github.com/onflow/flow-go-sdk"
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
	Success_Status = "SEALED"
)

// Bridge near bridge
type Bridge struct {
	*base.NonceSetterBase
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
		NonceSetterBase: base.NewNonceSetterBase(),
	}
}

// GetStubChainID get stub chainID
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("FLOW"))
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

// VerifyTokenConfig verify token config
func (b *Bridge) VerifyTokenConfig(tokenCfg *tokens.TokenConfig) error {
	return nil
}

// GetLatestBlockNumber gets latest block number
func (b *Bridge) GetLatestBlockNumber() (uint64, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetLatestBlock(url)
		if err == nil {
			return result.Height, nil
		}
	}
	return 0, tokens.ErrGetBlockNumberByID
}

func (b *Bridge) GetLatestBlockID() (sdk.Identifier, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetLatestBlock(url)
		if err == nil {
			return result.ID, nil
		}
	}
	return [32]byte{}, tokens.ErrGetBlockNumberByID
}

// GetLatestBlockNumberOf gets latest block number from single api
func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	result, err := GetLatestBlock(apiAddress)
	if err == nil {
		return result.Height, nil
	}
	return 0, tokens.ErrGetBlockNumberByID
}

func (b *Bridge) GetBlockNumberByHash(blockHash string) (uint64, error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := GetBlockNumberByHash(url, blockHash)
		if err == nil {
			return result, nil
		}
	}
	return 0, tokens.ErrGetBlockNumberByID
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionByHash get tx response by hash
func (b *Bridge) GetTransactionByHash(txHash string) (result *sdk.TransactionResult, err error) {
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err = GetTransactionByHash(url, txHash)
		if err == nil {
			return result, nil
		}
	}
	return nil, tokens.ErrTxNotFound
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)
	tx, err := b.GetTransaction(txHash)
	if err != nil {
		return nil, err
	}

	txres, ok := tx.(*sdk.TransactionResult)
	if !ok {
		// unexpected
		log.Warn("GetTransactionStatus", "error", errTxResultType)
		return nil, errTxResultType
	}

	// Check tx status
	if txres.Status.String() != Success_Status {
		log.Warn("Near tx status is not success", "result", txres.Status.String())
		return nil, tokens.ErrTxWithWrongStatus
	}

	status.Receipt = nil
	blockHeight, blockErr := b.GetBlockNumberByHash(txres.BlockID.Hex())
	if blockErr != nil {
		log.Warn("GetBlockNumberByHash", "error", blockErr)
		return nil, errTxResultType
	}
	status.BlockHeight = blockHeight

	if latest, err := b.GetLatestBlockNumber(); err == nil && latest > blockHeight {
		status.Confirmations = latest - blockHeight
	}
	return status, nil
}

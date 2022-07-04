package block

import (
	"math/big"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}

	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once
	blockClient           *Client

	cclis = make([]CoreClient, 0)
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
)

// Bridge near bridge
type Bridge struct {
	*tokens.CrossChainBridgeBase
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
	return &Bridge{CrossChainBridgeBase: tokens.NewCrossChainBridgeBase()}
}

// GetStubChainID get stub chainID
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("BLOCK"))
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

// GetLatestBlockNumberOf impl
func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	cli := b.GetClient()
	//# defer cli.Closer()
	for _, ccli := range cli.CClients {
		if ccli.Address == apiAddress {
			number, err := ccli.GetBlockCount()
			return uint64(number), err
		}
	}
	return 0, nil
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	txStatus := &tokens.TxStatus{}
	electStatus, err := b.GetElectTransactionStatus(txHash)
	if err != nil {
		log.Trace(b.ChainConfig.BlockChain+" Bridge::GetElectTransactionStatus fail", "tx", txHash, "err", err)
		return txStatus, err
	}
	if electStatus.BlockHash != nil {
		txStatus.BlockHash = *electStatus.BlockHash
	}
	if electStatus.BlockTime != nil {
		txStatus.BlockTime = *electStatus.BlockTime
	}
	if electStatus.BlockHeight != nil {
		txStatus.BlockHeight = *electStatus.BlockHeight
		latest, errt := b.GetLatestBlockNumber()
		if errt == nil {
			if latest > txStatus.BlockHeight {
				txStatus.Confirmations = latest - txStatus.BlockHeight
			}
		}
	}
	return txStatus, nil
}

func (b *Bridge) getTransactionByHashWithRetry(txid string) (tx *ElectTx, err error) {
	for i := 0; i < retryCount; i++ {
		tx, err = b.GetTransactionByHash(txid)
		if err == nil {
			break
		}
		time.Sleep(retryInterval)
	}
	return tx, err
}

// GetChainParams get chain config (net params)
func (b *Bridge) GetChainParams(chainID *big.Int) *chaincfg.Params {
	chainId := chainID.String()
	switch chainId {
	case (*GetStubChainID(mainnetNetWork)).String():
		return &chaincfg.MainNetParams
	default:
		return &chaincfg.MainNetParams
	}
}

// GetClient returns new Client
func (b *Bridge) GetClient() *Client {
	if blockClient != nil {
		return blockClient
	}

	cfg := b.GetGatewayConfig()
	if cfg.Extras == nil || cfg.Extras.BlockExtra == nil {
		return nil
	}

	if len(cclis) == 0 {
		for _, args := range cfg.Extras.BlockExtra.CoreAPIs {
			connCfg := &rpcclient.ConnConfig{
				Host:         args.APIAddress,
				User:         args.RPCUser,
				Pass:         args.RPCPassword,
				HTTPPostMode: true,            // Bitcoin core only supports HTTP POST mode
				DisableTLS:   args.DisableTLS, // Bitcoin core does not provide TLS by default
			}

			client, err := rpcclient.New(connCfg, nil)
			if err != nil {
				continue
			}

			ccli := CoreClient{
				Client:  client,
				Address: connCfg.Host,
				Closer:  client.Shutdown,
			}
			cclis = append(cclis, ccli)
		}
	}

	blockClient = &Client{
		CClients:         cclis,
		UTXOAPIAddresses: cfg.Extras.BlockExtra.UTXOAPIAddresses,
	}
	return blockClient
}

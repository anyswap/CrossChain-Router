package cosmosSDK

import (
	"math/big"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codecTypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	// ibcTypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
	// tokenfactoryTypes "github.com/sei-protocol/sei-chain/x/tokenfactory/types"
)

var (
	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once
	ChainsList            = []string{"COSMOSHUB", "SEI"}
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
)

func NewCosmosRestClient(urls []string, prefix, denom string) *CosmosRestClient {
	return &CosmosRestClient{
		BaseUrls: urls,
		TxConfig: BuildNewTxConfig(),
		Prefix:   prefix,
		Denom:    denom,
	}
}

func (c *CosmosRestClient) SetBaseUrls(urls []string) {
	c.BaseUrls = urls
}

func (c *CosmosRestClient) SetPrefixAndDenom(prefix, denom string) {
	c.Prefix = prefix
	c.Denom = denom
}

func BuildNewTxConfig() cosmosClient.TxConfig {
	interfaceRegistry := codecTypes.NewInterfaceRegistry()
	bankTypes.RegisterInterfaces(interfaceRegistry)
	// ibcTypes.RegisterInterfaces(interfaceRegistry)
	// tokenfactoryTypes.RegisterInterfaces(interfaceRegistry)
	PublicKeyRegisterInterfaces(interfaceRegistry)
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	return authTx.NewTxConfig(protoCodec, authTx.DefaultSignModes)
}

func PublicKeyRegisterInterfaces(registry codecTypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cryptoTypes.PubKey)(nil), &secp256k1.PubKey{})
}

// SupportsChainID supports chainID
func SupportsChainID(chainID *big.Int) bool {
	supportedChainIDsInit.Do(func() {
		for _, chainName := range ChainsList {
			supportedChainIDs[GetStubChainID(chainName, mainnetNetWork).String()] = true
			supportedChainIDs[GetStubChainID(chainName, testnetNetWork).String()] = true
		}
	})
	return supportedChainIDs[chainID.String()]
}

// GetStubChainID get stub chainID
func GetStubChainID(chainName, network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte(chainName))
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

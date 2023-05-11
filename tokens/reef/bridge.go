package reef

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
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

// Reef Bridge extends eth bridge
type Bridge struct {
	eth.Bridge
	WS            []ReefGraphQLImpl
	SubstrateAPIs []*gsrpc.SubstrateAPI
	MetaData      *types.Metadata
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	b := &Bridge{
		Bridge:        *eth.NewCrossChainBridge(),
		WS:            []ReefGraphQLImpl{},
		SubstrateAPIs: []*gsrpc.SubstrateAPI{},
	}
	b.Bridge.EvmContractBridge = b
	return b
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	b.CrossChainBridgeBase.InitAfterConfig()
	for _, url := range b.GatewayConfig.AllGatewayURLs {
		api, err := gsrpc.NewSubstrateAPI(url)
		if err != nil {
			panic(err)
		}
		b.SubstrateAPIs = append(b.SubstrateAPIs, api)
		if b.MetaData == nil {
			meta, err := api.RPC.State.GetMetadataLatest()
			if err != nil {
				panic(err)
			}
			b.MetaData = meta
		}
	}

	if len(b.WS) == 0 {
		b.InitGraphQL()
	}
	jspath := params.GetCustom(b.ChainConfig.ChainID, "jspath")
	if jspath == "" {
		panic(fmt.Errorf("%s not config jspath", b.ChainConfig.ChainID))
	}
	InstallJSModules(jspath, b.GatewayConfig.AllGatewayURLs[0])
}

func (b *Bridge) InitGraphQL() {
	wsnodes := strings.Split(params.GetCustom(b.ChainConfig.ChainID, "ws"), ",")
	if len(wsnodes) <= 0 {
		log.Warn("reef graphQL not config endpoint")
		return
	}
	for _, wsnode := range wsnodes {
		if strings.HasPrefix(wsnode, "wss") {
			ws, err := NewWebSocket(wsnode)
			if err != nil {
				log.Warn("reef websocket connect error", "chainid", b.ChainConfig.ChainID, "endpoint", wsnode)
				continue
			}
			go ws.Run()
			for i := 0; i < 30; i++ {
				time.Sleep(1 * time.Second)
				if !ws.IsClose {
					break
				}
			}
			if ws.IsClose {
				log.Warn("reef websocket connect error", "chainid", b.ChainConfig.ChainID, "endpoint", wsnode)
				continue
			} else {
				log.Info("session connect success", "chainid", b.ChainConfig.ChainID, "endpoint", wsnode)
			}
			b.WS = append(b.WS, ReefGraphQLImpl{
				WS:  ws,
				Uri: wsnode,
			})
		} else {
			b.WS = append(b.WS, ReefGraphQLImpl{
				Uri: wsnode,
			})
		}
	}
}

// SupportsChainID supports chainID
func SupportsChainID(chainID *big.Int) bool {
	supportedChainIDsInit.Do(func() {
		supportedChainIDs["13939"] = true
		supportedChainIDs[GetStubChainID(testnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(devnetNetWork).String()] = true
	})
	return supportedChainIDs[chainID.String()]
}

// GetStubChainID get stub chainID
// mainnet: 1001380271430
// testnet: 1001380271431
// devnet: 1001380271432
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("REEF"))
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

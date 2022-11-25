package aptos

import (
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
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

	rpcRetryTimes = 3
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

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	if params.IsSwapServer {
		InstallTsModules()
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
	stubChainID := new(big.Int).SetBytes([]byte("APT"))
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

// SetGatewayConfig set gateway config
func (b *Bridge) SetGatewayConfig(gatewayCfg *tokens.GatewayConfig) {
	b.CrossChainBridgeBase.SetGatewayConfig(gatewayCfg)
}

// SetTokenConfig set token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	if tokenCfg.RouterContract == "" {
		tokenCfg.RouterContract = b.ChainConfig.RouterContract
	}

	if tokens.IsERC20Router() {
		if b.IsNative(tokenAddr) {
			if tokenCfg.Decimals != 8 {
				log.Fatal("token decimals mismatch", "tokenID", tokenCfg.TokenID, "chainID", b.ChainConfig.ChainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract")
			}
		} else {
			decimals, errt := b.GetTokenDecimals(tokenAddr)
			if errt != nil {
				log.Fatal("get token decimals failed", "tokenAddr", tokenAddr, "tokenID", tokenCfg.TokenID, "chainID", b.ChainConfig.ChainID, "err", errt)
			}
			if decimals != tokenCfg.Decimals {
				log.Fatal("token decimals mismatch", "tokenID", tokenCfg.TokenID, "chainID", b.ChainConfig.ChainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
			}
		}
	}
	// aptos every coin has Extra, Extra is underlying if anytoken, Extra is ContractAddress when others

	if !strings.Contains(tokenCfg.Extra, SPLIT_SYMBOL) {
		tokenCfg.Extra = tokenCfg.ContractAddress
	}
	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)

	if !strings.EqualFold(tokenCfg.Extra, tokenAddr) {
		log.Info("init tokenCfg Extra", "Extra", tokenCfg.Extra, "tokenAddr", tokenAddr, "chainID", b.ChainConfig.ChainID)
		underlyingTokenConfig := *tokenCfg
		underlyingTokenConfig.ContractAddress = tokenCfg.Extra
		b.SetTokenConfig(tokenCfg.Extra, &underlyingTokenConfig)
	}

}

func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
	if routerContract == "" {
		return nil
	}
	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)

	routerMPCPubkey, err := router.GetMPCPubkey(routerContract)
	if err != nil {
		log.Fatal("get mpc public key failed", "mpc", routerContract, "err", err)
	}
	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC: routerContract,
		},
	)
	router.SetMPCPublicKey(routerContract, routerMPCPubkey)

	log.Info(fmt.Sprintf("[%5v] init routerContract success", chainID),
		"routerContract", routerContract, "routerMPCPubkey", routerMPCPubkey)

	return nil
}

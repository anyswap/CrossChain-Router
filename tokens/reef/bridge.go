package reef

import (
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	// _ tokens.IBridge = &Bridge{}

	// ensure Bridge impl tokens.NonceSetter
	// _ tokens.NonceSetter = &Bridge{}

	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
	devnetNetWork  = "devnet"
)

// Bridge reef bridge
type Bridge struct {
	*base.NonceSetterBase
	CustomConfig
	WS []*WebSocket
}

// CustomConfig custom config
type CustomConfig struct {
	// some chain's rpc is slow and need config a longer rpc timeout
	RPCClientTimeout int
	// eg. RSK chain do not check mixed case or not same as eth
	DontCheckAddressMixedCase bool
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CustomConfig:    NewCustomConfig(),
		NonceSetterBase: base.NewNonceSetterBase(),
		WS:              []*WebSocket{},
	}
}

// NewCustomConfig new custom config
func NewCustomConfig() CustomConfig {
	return CustomConfig{
		RPCClientTimeout:          client.GetDefaultTimeout(false),
		DontCheckAddressMixedCase: false,
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	wsnodes := strings.Split(params.GetCustom(b.ChainConfig.ChainID, "ws"), ",")
	if len(wsnodes) <= 0 {
		panic(fmt.Errorf("%s not config ws endpoint", b.ChainConfig.ChainID))
	}
	for _, wsnode := range wsnodes {
		ws, err := NewWebSocket(wsnode)
		if err != nil {
			log.Warn("reef websocket connect error", "chainid", b.ChainConfig.ChainID, "endpoint", wsnode)
			continue
		}
		go ws.Run()
		b.WS = append(b.WS, ws)
	}
	jspath := params.GetCustom(b.ChainConfig.ChainID, "ws")
	if jspath == "" {
		panic(fmt.Errorf("%s not config jspath", b.ChainConfig.ChainID))
	}
	InstallJSModules(params.GetCustom(b.ChainConfig.ChainID, "ws"))
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

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract string) (err error) {
	if routerContract == "" {
		return nil
	}

	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	var routerFactory, routerWNative, routerSecurity string
	if tokens.IsERC20Router() {
		routerFactory, err = b.GetFactoryAddress(routerContract)
		if err != nil {
			log.Warn("get router factory address failed", "chainID", chainID, "routerContract", routerContract, "err", err)
		}
		routerWNative, err = b.GetWNativeAddress(routerContract)
		if err != nil {
			log.Warn("get router wNative address failed", "chainID", chainID, "routerContract", routerContract, "err", err)
		}
		if params.GetSwapSubType() == "v7" {
			routerSecurity, err = b.GetRouterSecurity(routerContract)
			if err != nil {
				log.Warn("get router security address failed", "chainID", chainID, "routerContract", routerContract, "err", err)
				return err
			}
		}
	}
	routerMPC, err := b.GetMPCAddress(routerContract)
	if err != nil {
		log.Warn("get router mpc address failed", "chainID", chainID, "routerContract", routerContract, "err", err)
		return err
	}
	if common.HexToAddress(routerMPC) == (common.Address{}) {
		log.Warn("get router mpc address return an empty address", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
		return fmt.Errorf("empty router mpc address of router contract %v on chain %v", routerContract, chainID)
	}
	if !b.IsValidAddress(routerMPC) {
		log.Warn("wrong router mpc address", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
		return fmt.Errorf("wrong router mpc address '%v' of router contract %v on chain %v", routerMPC, routerContract, chainID)
	}
	log.Info("get router mpc address success", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Warn("get mpc public key failed", "chainID", chainID, "mpc", routerMPC, "err", err)
		return err
	}
	if err = VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Warn("verify mpc public key failed", "chainID", chainID, "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
		return err
	}
	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC:      routerMPC,
			RouterFactory:  routerFactory,
			RouterWNative:  routerWNative,
			RouterSecurity: routerSecurity,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID),
		"routerContract", routerContract, "routerMPC", routerMPC,
		"routerFactory", routerFactory, "routerWNative", routerWNative,
		"routerSecurity", routerSecurity)

	return nil
}

// SetTokenConfig set token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)

	if tokenCfg == nil || !tokens.IsERC20Router() {
		return
	}

	tokenID := tokenCfg.TokenID
	chainID := b.ChainConfig.ChainID

	if tokenCfg.ContractVersion >= eth.MintBurnWrapperTokenVersion {
		log.Info("ignore wrapper token config checking",
			"chainID", chainID, "tokenID", tokenID, "tokenAddr", tokenAddr,
			"decimals", tokenCfg.Decimals, "ContractVersion", tokenCfg.ContractVersion)
		return
	}

	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)

	decimals, errt := b.GetErc20Decimals(tokenAddr)
	if errt != nil {
		logErrFunc("get token decimals failed", "chainID", chainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "err", errt)
		return
	}
	if decimals != tokenCfg.Decimals {
		logErrFunc("token decimals mismatch", "chainID", chainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
		return
	}
	routerContract := tokenCfg.RouterContract
	if routerContract == "" {
		routerContract = b.GetChainConfig().RouterContract
	}
	err := b.checkTokenMinter(routerContract, tokenCfg)
	if err != nil && tokenCfg.IsStandardTokenVersion() {
		logErrFunc("check token minter failed", "chainID", chainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "err", err)
		return
	}
	underlying, err := b.GetUnderlyingAddress(tokenAddr)
	if err != nil && tokenCfg.IsStandardTokenVersion() {
		logErrFunc("get underlying address failed", "chainID", chainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "err", err)
		return
	}
	var underlyingIsMinted bool
	if common.HexToAddress(underlying) != (common.Address{}) {
		for i := 0; i < 2; i++ {
			underlyingIsMinted, err = b.IsUnderlyingMinted(tokenAddr)
			if err == nil {
				break
			}
		}
	}
	tokenCfg.SetUnderlying(underlying, underlyingIsMinted) // init underlying address
}

func (b *Bridge) checkTokenMinter(routerContract string, tokenCfg *tokens.TokenConfig) (err error) {
	if !tokenCfg.IsStandardTokenVersion() {
		return nil
	}
	tokenAddr := tokenCfg.ContractAddress
	var minterAddr string
	var isMinter bool
	switch tokenCfg.ContractVersion {
	default:
		isMinter, err = b.IsMinter(tokenAddr, routerContract)
		if err != nil {
			return err
		}
		if !isMinter {
			return fmt.Errorf("%v is not minter", routerContract)
		}
		return nil
	case 3:
		minterAddr, err = b.GetVaultAddress(tokenAddr)
	case 2, 1:
		minterAddr, err = b.GetOwnerAddress(tokenAddr)
	}
	if err != nil {
		return err
	}
	if common.HexToAddress(minterAddr) != common.HexToAddress(routerContract) {
		return fmt.Errorf("minter mismatch, have '%v' config '%v'", minterAddr, routerContract)
	}
	return nil
}

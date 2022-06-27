package tron

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
)

var TronMainnetChainID = uint64(112233)
var TronShastaChainID = uint64(2494104990)

// Bridge eth bridge
type Bridge struct {
	CustomConfig
	*tokens.CrossChainBridgeBase
	SignerChainID *big.Int
	TronChainID   *big.Int
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CustomConfig:         NewCustomConfig(),
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
	}
}

// CustomConfig custom config
type CustomConfig struct {
	// some chain's rpc is slow and need config a longer rpc timeout
	RPCClientTimeout int
	// eg. RSK chain do not check mixed case or not same as eth
	DontCheckAddressMixedCase bool
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
	chainID, err := common.GetBigIntFromStr(b.ChainConfig.ChainID)
	if err != nil {
		log.Fatal("wrong chainID", "chainID", b.ChainConfig.ChainID, "blockChain", b.ChainConfig.BlockChain)
	}
	switch chainID.Uint64() {
	case TronMainnetChainID, TronShastaChainID:
		b.TronChainID = chainID
	default:
		log.Fatal("wrong chainID")
	}
	b.InitExtraCustoms()
}

// InitGatewayConfig impl
func (b *Bridge) InitGatewayConfig(chainID *big.Int) {
	if chainID.Sign() == 0 {
		log.Fatal("zero chain ID")
	}
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	b.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	latestBlock, err := b.GetLatestBlockNumber()
	if err != nil && router.IsIniting {
		for i := 0; i < router.RetryRPCCountInInit; i++ {
			if latestBlock, err = b.GetLatestBlockNumber(); err == nil {
				break
			}
			time.Sleep(router.RetryRPCIntervalInInit)
		}
	}
	if err != nil {
		log.Fatal("get lastest block number failed", "chainID", chainID, "err", err)
	}
	log.Infof("[%5v] lastest block number is %v", chainID, latestBlock)
	log.Infof("[%5v] init gateway config success", chainID)
}

// SetTokenConfig set token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	tokenCfg.ContractAddress = anyToEth(tokenCfg.ContractAddress)
	tokenCfg.RouterContract = anyToEth(tokenCfg.RouterContract)
	tokenAddr = anyToEth(tokenAddr)
	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)

	if tokenCfg == nil || !tokens.IsERC20Router() {
		return
	}
}

// InitChainConfig impl
func (b *Bridge) InitChainConfig(chainID *big.Int) {
	chainCfg, err := router.GetChainConfig(chainID)
	if err != nil {
		log.Fatal("get chain config failed", "chainID", chainID, "err", err)
	}
	if chainCfg == nil {
		log.Fatal("chain config not found", "chainID", chainID)
	}
	if chainID.String() != chainCfg.ChainID {
		log.Fatal("verify chain ID mismatch", "inconfig", chainCfg.ChainID, "inchainids", chainID)
	}
	if err = chainCfg.CheckConfig(); err != nil {
		log.Fatal("check chain config failed", "chainID", chainID, "err", err)
	}
	if err = b.InitRouterInfo(chainCfg.RouterContract); err != nil {
		log.Fatal("init chain router info failed", "routerContract", chainCfg.RouterContract, "err", err)
	}
	b.SetChainConfig(chainCfg)
	log.Info("init chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID)
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract string) (err error) {
	if routerContract == "" {
		return nil
	}
	var routerFactory, routerWNative string
	if tokens.IsERC20Router() {
		routerFactory, err = b.GetFactoryAddress(routerContract)
		if err != nil {
			log.Warn("get router factory address failed", "routerContract", routerContract, "err", err)
		}
		routerWNative, err = b.GetWNativeAddress(routerContract)
		if err != nil {
			log.Warn("get router wNative address failed", "routerContract", routerContract, "err", err)
		}
	}
	routerMPC, err := b.GetMPCAddress(routerContract)
	if err != nil {
		log.Warn("get router mpc address failed", "routerContract", routerContract, "err", err)
		return err
	}
	if common.HexToAddress(routerMPC) == (common.Address{}) {
		log.Warn("get router mpc address return an empty address", "routerContract", routerContract)
		return fmt.Errorf("empty router mpc address")
	}
	log.Info("get router mpc address success", "routerContract", routerContract, "routerMPC", routerMPC)
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Warn("get mpc public key failed", "mpc", routerMPC, "err", err)
		return err
	}
	if err = VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Warn("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
		return err
	}
	chainID := b.ChainConfig.ChainID
	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC:     routerMPC,
			RouterFactory: routerFactory,
			RouterWNative: routerWNative,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID),
		"routerContract", routerContract, "routerMPC", routerMPC,
		"routerFactory", routerFactory, "routerWNative", routerWNative)

	if mongodb.HasClient() {
		nextSwapNonce, err := mongodb.FindNextSwapNonce(chainID, strings.ToLower(routerMPC))
		if err == nil {
			log.Info("init next swap nonce from db", "chainID", chainID, "mpc", routerMPC, "nonce", nextSwapNonce)
		}
	}

	return nil
}

// ReloadChainConfig reload chain config
func (b *Bridge) ReloadChainConfig(chainID *big.Int) {
	chainCfg, err := router.GetChainConfig(chainID)
	if err != nil {
		log.Error("[reload] get chain config failed", "chainID", chainID, "err", err)
		return
	}
	if chainCfg == nil {
		log.Error("[reload] chain config not found", "chainID", chainID)
		return
	}
	if chainID.String() != chainCfg.ChainID {
		log.Error("[reload] verify chain ID mismatch", "inconfig", chainCfg.ChainID, "inchainids", chainID)
		return
	}
	if err = chainCfg.CheckConfig(); err != nil {
		log.Error("[reload] check chain config failed", "chainID", chainID, "err", err)
		return
	}
	if err = b.InitRouterInfo(chainCfg.RouterContract); err != nil {
		log.Error("init chain router info failed", "routerContract", chainCfg.RouterContract, "err", err)
		return
	}
	b.SetChainConfig(chainCfg)
	log.Info("reload chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID)
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

// GetSignerChainID default way to get signer chain id
// use chain ID first, if missing then use network ID instead.
// normally this way works, but sometimes it failed (eg. ETC),
// then we should overwrite this function
// NOTE: call after chain config setted
func (b *Bridge) GetSignerChainID() (*big.Int, error) {
	switch strings.ToUpper(b.ChainConfig.BlockChain) {
	default:
		chainID, err := b.ChainID()
		if err != nil {
			return nil, err
		}
		if chainID.Sign() != 0 {
			return chainID, nil
		}
		return b.NetworkID()
	case "ETHCLASSIC":
		return b.getETCSignerChainID()
	}
}

func (b *Bridge) getETCSignerChainID() (*big.Int, error) {
	networkID, err := b.NetworkID()
	if err != nil {
		return nil, err
	}
	var chainID uint64
	switch networkID.Uint64() {
	case 1:
		chainID = 61 // mainnet
	case 6:
		chainID = 6 // kotti
	case 7:
		chainID = 63 // mordor
	default:
		log.Warnf("unsupported etc network id '%v'", networkID)
		return nil, errors.New("unsupported etc network id")
	}
	return new(big.Int).SetUint64(chainID), nil
}

// InitExtraCustoms init extra customs
func (b *Bridge) InitExtraCustoms() {
	clientTimeout := params.GetRPCClientTimeout(b.ChainConfig.ChainID)
	if clientTimeout != 0 {
		b.RPCClientTimeout = clientTimeout
	} else {
		timeoutStr := params.GetCustom(b.ChainConfig.ChainID, "sendtxTimeout")
		if timeoutStr != "" {
			timeout, err := common.GetUint64FromStr(timeoutStr)
			if err != nil {
				log.Fatal("get sendtxTimeout failed", "err", err)
			}
			if timeout != 0 {
				b.RPCClientTimeout = int(timeout)
			}
		}
	}
	flag := params.GetCustom(b.ChainConfig.ChainID, "dontCheckAddressMixedCase")
	b.DontCheckAddressMixedCase = strings.EqualFold(flag, "true")
}

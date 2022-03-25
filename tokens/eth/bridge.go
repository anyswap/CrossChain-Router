package eth

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
	"github.com/anyswap/CrossChain-Router/v3/types"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.NonceSetter
	_ tokens.NonceSetter = &Bridge{}
)

// Bridge eth bridge
type Bridge struct {
	CustomConfig
	*tokens.CrossChainBridgeBase
	*NonceSetterBase
	Signer        types.Signer
	SignerChainID *big.Int
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CustomConfig:         NewCustomConfig(),
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
		NonceSetterBase:      NewNonceSetterBase(),
	}
}

// CustomConfig custom config
type CustomConfig struct {
	// some chain's rpc is slow and need config a longer rpc timeout
	SendtxTimeout int
	// eg. RSK chain do not check mixed case or not same as eth
	DontCheckAddressMixedCase bool
}

// NewCustomConfig new custom config
func NewCustomConfig() CustomConfig {
	return CustomConfig{
		SendtxTimeout:             client.GetDefaultTimeout(false),
		DontCheckAddressMixedCase: false,
	}
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	chainID, err := common.GetBigIntFromStr(b.ChainConfig.ChainID)
	if err != nil {
		log.Fatal("wrong chainID", "chainID", b.ChainConfig.ChainID, "blockChain", b.ChainConfig.BlockChain)
	}
	b.InitExtraCustoms()
	b.initSigner(chainID)
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
	if err = b.InitRouterInfo(chainID, chainCfg.RouterContract); err != nil {
		log.Fatal("init chain router info failed", "routerContract", chainCfg.RouterContract, "err", err)
	}
	b.SetChainConfig(chainCfg)
	log.Info("init chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID)
}

func (b *Bridge) initSigner(chainID *big.Int) {
	signerChainID, err := b.GetSignerChainID()
	if err != nil && router.IsIniting {
		for i := 0; i < router.RetryRPCCountInInit; i++ {
			if signerChainID, err = b.GetSignerChainID(); err == nil {
				break
			}
			time.Sleep(router.RetryRPCIntervalInInit)
		}
	}
	if err != nil {
		log.Fatal("get signer chain ID failed", "chainID", chainID, "err", err)
	}
	if chainID.Cmp(signerChainID) != 0 {
		log.Fatal("chain ID mismatch", "inconfig", chainID, "inbridge", signerChainID)
	}
	b.SignerChainID = signerChainID
	if params.IsDynamicFeeTxEnabled(signerChainID.String()) {
		b.Signer = types.MakeSigner("London", signerChainID)
	} else {
		b.Signer = types.MakeSigner("EIP155", signerChainID)
	}
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(biChainID *big.Int, routerContract string) (err error) {
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
	router.SetRouterInfo(
		routerContract,
		&router.SwapRouterInfo{
			RouterMPC:     routerMPC,
			RouterFactory: routerFactory,
			RouterWNative: routerWNative,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)

	chainID := biChainID.String()

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID),
		"routerContract", routerContract, "routerMPC", routerMPC,
		"routerFactory", routerFactory, "routerWNative", routerWNative)

	if mongodb.HasClient() {
		nextSwapNonce, err := mongodb.FindNextSwapNonce(chainID, strings.ToLower(routerMPC))
		if err == nil {
			log.Info("init next swap nonce from db", "chainID", chainID, "mpc", routerMPC, "nonce", nextSwapNonce)
			b.InitSwapNonce(routerMPC, nextSwapNonce)
		}
	}

	return nil
}

// InitTokenConfig impl
func (b *Bridge) InitTokenConfig(tokenID string, chainID *big.Int) {
	if tokenID == "" {
		log.Fatal("empty token ID")
	}
	tokenAddr, err := router.GetMultichainToken(tokenID, chainID)
	if err != nil {
		log.Fatal("get token address failed", "tokenID", tokenID, "chainID", chainID, "err", err)
	}
	if common.HexToAddress(tokenAddr) == (common.Address{}) {
		log.Debugf("[%5v] '%v' token address is empty", chainID, tokenID)
		return
	}
	tokenCfg, err := router.GetTokenConfig(chainID, tokenID)
	if err != nil {
		log.Fatal("get token config failed", "chainID", chainID, "tokenID", tokenID, "err", err)
	}
	if tokenCfg == nil {
		log.Debug("token config not found", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr)
		return
	}
	if common.HexToAddress(tokenAddr) != common.HexToAddress(tokenCfg.ContractAddress) {
		log.Fatal("verify token address mismach", "tokenID", tokenID, "chainID", chainID, "inconfig", tokenCfg.ContractAddress, "inmultichain", tokenAddr)
	}
	if tokenID != tokenCfg.TokenID {
		log.Fatal("verify token ID mismatch", "chainID", chainID, "inconfig", tokenCfg.TokenID, "intokenids", tokenID)
	}
	if err = tokenCfg.CheckConfig(); err != nil {
		log.Fatal("check token config failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
	}
	routerContract, err := router.GetCustomConfig(chainID, tokenAddr)
	if err != nil {
		log.Fatal("get custom config failed", "chainID", chainID, "key", tokenAddr, "err", err)
	}
	tokenCfg.RouterContract = routerContract
	if routerContract == "" {
		routerContract = b.ChainConfig.RouterContract
	}
	if err = b.InitRouterInfo(chainID, tokenCfg.RouterContract); err != nil {
		log.Fatal("init token router info failed", "routerContract", tokenCfg.RouterContract, "err", err)
	}

	var underlying string
	if tokens.IsERC20Router() {
		decimals, errt := b.GetErc20Decimals(tokenAddr)
		if errt != nil {
			log.Fatal("get token decimals failed", "tokenAddr", tokenAddr, "err", errt)
		}
		if decimals != tokenCfg.Decimals {
			log.Fatal("token decimals mismatch", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
		}
		err = b.checkTokenMinter(routerContract, tokenCfg)
		if err != nil && tokenCfg.IsStandardTokenVersion() {
			log.Fatal("check token minter failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
		}
		underlying, err = b.GetUnderlyingAddress(tokenAddr)
		if err != nil && tokenCfg.IsStandardTokenVersion() {
			log.Fatal("get underlying address failed", "err", err)
		}
		tokenCfg.SetUnderlying(common.HexToAddress(underlying)) // init underlying address
	}

	b.SetTokenConfig(tokenAddr, tokenCfg)

	tokenIDKey := strings.ToLower(tokenID)
	tokensMap := router.MultichainTokens[tokenIDKey]
	if tokensMap == nil {
		tokensMap = make(map[string]string)
		router.MultichainTokens[tokenIDKey] = tokensMap
	}
	tokensMap[chainID.String()] = tokenAddr

	log.Info(fmt.Sprintf("[%5v] init '%v' token config success", chainID, tokenID),
		"tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals, "underlying", underlying)
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
	if err = b.InitRouterInfo(chainID, chainCfg.RouterContract); err != nil {
		log.Error("init chain router info failed", "routerContract", chainCfg.RouterContract, "err", err)
		return
	}
	b.SetChainConfig(chainCfg)
	log.Info("reload chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID)
}

// ReloadTokenConfig reload token config
func (b *Bridge) ReloadTokenConfig(tokenID string, chainID *big.Int) {
	if tokenID == "" {
		return
	}
	tokenAddr, err := router.GetMultichainToken(tokenID, chainID)
	if err != nil {
		log.Error("[reload] get token address failed", "tokenID", tokenID, "chainID", chainID, "err", err)
		return
	}
	if common.HexToAddress(tokenAddr) == (common.Address{}) {
		log.Debug("[reload] multichain token address is empty", "tokenID", tokenID, "chainID", chainID)
		return
	}
	tokenCfg, err := router.GetTokenConfig(chainID, tokenID)
	if err != nil {
		log.Error("[reload] get token config failed", "chainID", chainID, "tokenID", tokenID, "err", err)
		return
	}
	if tokenCfg == nil {
		log.Debug("[reload] token config not found", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr)
		return
	}
	if common.HexToAddress(tokenAddr) != common.HexToAddress(tokenCfg.ContractAddress) {
		log.Error("[reload] verify token address mismach", "tokenID", tokenID, "chainID", chainID, "inconfig", tokenCfg.ContractAddress, "inmultichain", tokenAddr)
		return
	}
	if tokenID != tokenCfg.TokenID {
		log.Error("[reload] verify token ID mismatch", "chainID", chainID, "inconfig", tokenCfg.TokenID, "intokenids", tokenID)
		return
	}
	if err = tokenCfg.CheckConfig(); err != nil {
		log.Error("[reload] check token config failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
		return
	}
	routerContract, err := router.GetCustomConfig(chainID, tokenAddr)
	if err != nil {
		log.Error("get custom config failed", "chainID", chainID, "key", tokenAddr, "err", err)
		return
	}
	tokenCfg.RouterContract = routerContract
	if routerContract == "" {
		routerContract = b.ChainConfig.RouterContract
	}
	if err = b.InitRouterInfo(chainID, tokenCfg.RouterContract); err != nil {
		log.Error("init token router info failed", "routerContract", tokenCfg.RouterContract, "err", err)
		return
	}

	var underlying string
	if tokens.IsERC20Router() {
		decimals, errt := b.GetErc20Decimals(tokenAddr)
		if errt != nil {
			log.Error("[reload] get token decimals failed", "tokenAddr", tokenAddr, "err", errt)
			return
		}
		if decimals != tokenCfg.Decimals {
			log.Error("[reload] token decimals mismatch", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
			return
		}
		err = b.checkTokenMinter(routerContract, tokenCfg)
		if err != nil && tokenCfg.IsStandardTokenVersion() {
			log.Error("[reload] check token minter failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
			return
		}
		underlying, err = b.GetUnderlyingAddress(tokenAddr)
		if err != nil && tokenCfg.IsStandardTokenVersion() {
			log.Error("[reload] get underlying address failed", "err", err)
			return
		}
		tokenCfg.SetUnderlying(common.HexToAddress(underlying)) // init underlying address
	}

	b.SetTokenConfig(tokenAddr, tokenCfg)

	tokenIDKey := strings.ToLower(tokenID)
	tokensMap := router.MultichainTokens[tokenIDKey]
	if tokensMap == nil {
		tokensMap = make(map[string]string)
		router.MultichainTokens[tokenIDKey] = tokensMap
	}
	tokensMap[chainID.String()] = tokenAddr

	log.Info("reload token config success", "chainID", chainID, "tokenID", tokenID,
		"tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals, "underlying", underlying)
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
	timeoutStr := params.GetCustom(b.ChainConfig.ChainID, "sendtxTimeout")
	if timeoutStr != "" {
		timeout, err := common.GetUint64FromStr(timeoutStr)
		if err != nil {
			log.Fatal("get sendtxTimeout failed", "err", err)
		}
		if timeout != 0 {
			b.SendtxTimeout = int(timeout)
		}
	}
	flag := params.GetCustom(b.ChainConfig.ChainID, "dontCheckAddressMixedCase")
	b.DontCheckAddressMixedCase = strings.EqualFold(flag, "true")
}

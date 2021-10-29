package eth

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
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
	*tokens.CrossChainBridgeBase
	*NonceSetterBase
	Signer        types.Signer
	SignerChainID *big.Int

	// eg. RSK chain do not check mixed case or not same as eth
	DontCheckAddressMixedCase bool
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
		NonceSetterBase:      NewNonceSetterBase(),
	}
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
	if err != nil {
		log.Fatal("get lastest block number failed", "chainID", chainID, "err", err)
	}
	log.Infof(">>> [%5v] lastest block number is %v", chainID, latestBlock)
	log.Infof(">>> [%5v] init gateway config success", chainID)
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
	var routerFactory, routerWNative string
	if tokens.IsERC20Router() {
		routerFactory, err = b.GetFactoryAddress(chainCfg.RouterContract)
		if err != nil {
			log.Warn("get router factory address failed", "routerContract", chainCfg.RouterContract, "err", err)
		}
		routerWNative, err = b.GetWNativeAddress(chainCfg.RouterContract)
		if err != nil {
			log.Warn("get router wNative address failed", "routerContract", chainCfg.RouterContract, "err", err)
		}
		chainCfg.SetRouterFactory(routerFactory)
		chainCfg.SetRouterWNative(routerWNative)
	}
	routerMPC, err := b.GetMPCAddress(chainCfg.RouterContract)
	if err != nil {
		log.Fatal("get router mpc address failed", "routerContract", chainCfg.RouterContract, "err", err)
	}
	if common.HexToAddress(routerMPC) == (common.Address{}) {
		log.Fatal("get router mpc address return an empty address", "routerContract", chainCfg.RouterContract)
	}
	log.Info("get router mpc address success", "routerContract", chainCfg.RouterContract, "routerMPC", routerMPC)
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Fatal("get mpc public key failed", "mpc", routerMPC, "err", err)
	}
	if err = tokens.VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Fatal("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
	}
	chainCfg.SetRouterMPC(routerMPC)
	chainCfg.SetRouterMPCPubkey(routerMPCPubkey)
	b.SetChainConfig(chainCfg)
	b.initSigner(chainID)
	log.Info("init chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID,
		"routerContract", chainCfg.RouterContract, "routerMPC", routerMPC,
		"routerFactory", routerFactory, "routerWNative", routerWNative)
	log.Infof(">>> [%5v] init chain config success. router contract is %v, mpc address is %v", chainID, chainCfg.RouterContract, routerMPC)

	if mongodb.HasClient() {
		nextSwapNonce, err := mongodb.FindNextSwapNonce(chainID.String(), strings.ToLower(routerMPC))
		if err == nil {
			log.Info("init next swap nonce from db", "chainID", chainID, "mpc", routerMPC, "nonce", nextSwapNonce)
			b.SwapNonce[routerMPC] = nextSwapNonce
		}
	}
}

func (b *Bridge) initSigner(chainID *big.Int) {
	signerChainID, err := b.GetSignerChainID()
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
		log.Debugf(">>> [%5v] '%v' token address is empty", chainID, tokenID)
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
	var underlying string
	if tokens.IsERC20Router() {
		decimals, err := b.GetErc20Decimals(tokenAddr)
		if err != nil {
			log.Fatal("get token decimals failed", "tokenAddr", tokenAddr, "err", err)
		}
		if decimals != tokenCfg.Decimals {
			log.Fatal("token decimals mismatch", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
		}
		if err = b.checkTokenMinter(tokenAddr, tokenCfg.ContractVersion); err != nil {
			log.Fatal("check token minter failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
		}
		underlying, err = b.GetUnderlyingAddress(tokenAddr)
		if err != nil {
			log.Fatal("get underlying address failed", "err", err)
		}
		tokenCfg.SetUnderlying(common.HexToAddress(underlying)) // init underlying address
	}
	b.SetTokenConfig(tokenAddr, tokenCfg)
	log.Info(fmt.Sprintf(">>> [%5v] init '%v' token config success", chainID, tokenID), "tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals, "underlying", underlying)

	tokenIDKey := strings.ToLower(tokenID)
	tokensMap := router.MultichainTokens[tokenIDKey]
	if tokensMap == nil {
		tokensMap = make(map[string]string)
		router.MultichainTokens[tokenIDKey] = tokensMap
	}
	tokensMap[chainID.String()] = tokenAddr
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
	var routerFactory, routerWNative string
	if tokens.IsERC20Router() {
		routerFactory, err = b.GetFactoryAddress(chainCfg.RouterContract)
		if err != nil {
			log.Warn("[reload] get router factory address failed", "routerContract", chainCfg.RouterContract, "err", err)
		}
		routerWNative, err = b.GetWNativeAddress(chainCfg.RouterContract)
		if err != nil {
			log.Warn("get router wNative address failed", "routerContract", chainCfg.RouterContract, "err", err)
		}
		chainCfg.SetRouterFactory(routerFactory)
		chainCfg.SetRouterWNative(routerWNative)
	}
	routerMPC, err := b.GetMPCAddress(chainCfg.RouterContract)
	if err != nil {
		log.Error("[reload] get router mpc address failed", "routerContract", chainCfg.RouterContract, "err", err)
		return
	}
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Error("[reload] get mpc public key failed", "mpc", routerMPC, "err", err)
		return
	}
	if err = tokens.VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Error("[reload] verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
		return
	}
	chainCfg.SetRouterMPC(routerMPC)
	chainCfg.SetRouterMPCPubkey(routerMPCPubkey)
	b.SetChainConfig(chainCfg)
	log.Info("reload chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID,
		"routerContract", chainCfg.RouterContract, "routerMPC", routerMPC,
		"routerFactory", routerFactory, "routerWNative", routerWNative)
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
	var underlying string
	if tokens.IsERC20Router() {
		decimals, err := b.GetErc20Decimals(tokenAddr)
		if err != nil {
			log.Error("[reload] get token decimals failed", "tokenAddr", tokenAddr, "err", err)
			return
		}
		if decimals != tokenCfg.Decimals {
			log.Error("[reload] token decimals mismatch", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
			return
		}
		if err = b.checkTokenMinter(tokenAddr, tokenCfg.ContractVersion); err != nil {
			log.Error("[reload] check token minter failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
			return
		}
		underlying, err = b.GetUnderlyingAddress(tokenAddr)
		if err != nil {
			log.Error("[reload] get underlying address failed", "err", err)
			return
		}
		tokenCfg.SetUnderlying(common.HexToAddress(underlying)) // init underlying address
	}
	b.SetTokenConfig(tokenAddr, tokenCfg)
	log.Info("reload token config success", "chainID", chainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals, "underlying", underlying)

	tokenIDKey := strings.ToLower(tokenID)
	tokensMap := router.MultichainTokens[tokenIDKey]
	if tokensMap == nil {
		tokensMap = make(map[string]string)
		router.MultichainTokens[tokenIDKey] = tokensMap
	}
	tokensMap[chainID.String()] = tokenAddr
}

func (b *Bridge) checkTokenMinter(tokenAddr string, tokenVer uint64) (err error) {
	routerContract := b.ChainConfig.RouterContract
	var minterAddr string
	var isMinter bool
	switch tokenVer {
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
	case 0:
		return errors.New("token version is 0")
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

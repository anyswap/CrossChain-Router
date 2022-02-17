package solana

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	routerprog "github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}

	routerPDASeeds = [][]byte{[]byte("Router")}
)

// Bridge solana bridge
type Bridge struct {
	*tokens.CrossChainBridgeBase
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
	}
}

// SupportChainID support chainID
func SupportChainID(chainID *big.Int) bool {
	chainIDNum := chainID.Uint64()
	return chainIDNum == 245022934 || // mainnet
		chainIDNum == 245022940 || // testnet
		chainIDNum == 245022926 // devnet
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
	b.SetChainConfig(chainCfg)
	log.Info("init chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID)
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
	if tokenAddr != tokenCfg.ContractAddress {
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
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		log.Fatal("get router contract pubkey failed", "routerContract", routerContract, "err", err)
		return
	}
	routerPDAPubkey, bump, err := types.PublicKeyFindProgramAddress(routerPDASeeds, routerContractPubkey)
	if err != nil {
		log.Fatal("get router pda failed", "seeds", routerPDASeeds, "routerContract", routerContract, "err", err)
		return
	}
	routerPDA := routerPDAPubkey.String()

	if tokens.IsERC20Router() {
		decimals, errt := b.GetTokenDecimals(tokenAddr)
		if errt != nil {
			log.Fatal("get token decimals failed", "tokenAddr", tokenAddr, "err", errt)
		}
		if decimals != tokenCfg.Decimals {
			log.Fatal("token decimals mismatch", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
		}
		err = b.checkTokenMinter(routerPDA, tokenCfg)
		if err != nil && tokenCfg.IsStandardTokenVersion() {
			log.Fatal("check token minter failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
		}
	}
	b.SetTokenConfig(tokenAddr, tokenCfg)

	tokenIDKey := strings.ToLower(tokenID)
	tokensMap := router.MultichainTokens[tokenIDKey]
	if tokensMap == nil {
		tokensMap = make(map[string]string)
		router.MultichainTokens[tokenIDKey] = tokensMap
	}
	tokensMap[chainID.String()] = tokenAddr

	routerAccount, err := b.GetRouterAccount(routerContract)
	if err != nil {
		log.Fatal("get router account failed", "routerContract", routerContract, "err", err)
	}
	if routerAccount.Bump != bump {
		log.Fatal("get router account bump mismatch", "routerContract", routerContract, "have", routerAccount.Bump, "want", bump)
	}
	log.Info("get router account success", "routerContract", routerContract, "routerAccount", routerAccount)
	routerMPC := routerAccount.MPC.String()
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Fatal("get mpc public key failed", "mpc", routerMPC, "err", err)
	}
	if err = VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Fatal("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
	}

	router.SetRouterInfo(
		routerContract,
		&router.SwapRouterInfo{
			RouterMPC: routerMPC,
			RouterPDA: routerPDA,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)
	routerprog.InitRouterProgram(routerContractPubkey)

	log.Info(fmt.Sprintf("[%5v] init '%v' token config success", chainID, tokenID),
		"tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals,
		"routerContract", routerContract, "routerMPC", routerMPC, "routerPDA", routerPDA)
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
	if tokenAddr != tokenCfg.ContractAddress {
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
		log.Error("[reload] get custom config failed", "chainID", chainID, "key", tokenAddr, "err", err)
		return
	}
	tokenCfg.RouterContract = routerContract
	if routerContract == "" {
		routerContract = b.ChainConfig.RouterContract
	}
	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		log.Error("[reload] get router contract pubkey failed", "routerContract", routerContract, "err", err)
		return
	}
	routerPDAPubkey, bump, err := types.PublicKeyFindProgramAddress(routerPDASeeds, routerContractPubkey)
	if err != nil {
		log.Error("[reload] get router pda failed", "seeds", routerPDASeeds, "routerContract", routerContract, "err", err)
		return
	}
	routerPDA := routerPDAPubkey.String()

	if tokens.IsERC20Router() {
		decimals, errt := b.GetTokenDecimals(tokenAddr)
		if errt != nil {
			log.Error("[reload] get token decimals failed", "tokenAddr", tokenAddr, "err", errt)
			return
		}
		if decimals != tokenCfg.Decimals {
			log.Error("[reload] token decimals mismatch", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
			return
		}
		err = b.checkTokenMinter(routerPDA, tokenCfg)
		if err != nil && tokenCfg.IsStandardTokenVersion() {
			log.Error("[reload] check token minter failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
			return
		}
	}
	b.SetTokenConfig(tokenAddr, tokenCfg)

	tokenIDKey := strings.ToLower(tokenID)
	tokensMap := router.MultichainTokens[tokenIDKey]
	if tokensMap == nil {
		tokensMap = make(map[string]string)
		router.MultichainTokens[tokenIDKey] = tokensMap
	}
	tokensMap[chainID.String()] = tokenAddr

	routerAccount, err := b.GetRouterAccount(routerContract)
	if err != nil {
		log.Error("[reload] get router account failed", "routerContract", routerContract, "err", err)
	}
	if routerAccount.Bump != bump {
		log.Fatal("get router account bump mismatch", "routerContract", routerContract, "have", routerAccount.Bump, "want", bump)
	}
	log.Info("[reload] get router account success", "routerContract", routerContract, "routerAccount", routerAccount)
	routerMPC := routerAccount.MPC.String()
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Fatal("[reload] get mpc public key failed", "mpc", routerMPC, "err", err)
	}
	if err = VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Fatal("[reload] verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
	}

	router.SetRouterInfo(
		routerContract,
		&router.SwapRouterInfo{
			RouterMPC: routerMPC,
			RouterPDA: routerPDA,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)
	routerprog.InitRouterProgram(routerContractPubkey)

	log.Info("reload token config success", "chainID", chainID, "tokenID", tokenID,
		"tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals,
		"routerContract", routerContract, "routerMPC", routerMPC, "routerPDA", routerPDA)
}

func (b *Bridge) checkTokenMinter(routerPDA string, tokenCfg *tokens.TokenConfig) (err error) {
	if !tokenCfg.IsStandardTokenVersion() {
		return nil
	}
	tokenAddr := tokenCfg.ContractAddress
	isMinter, err := b.IsMinter(tokenAddr, routerPDA)
	if err != nil {
		return err
	}
	if !isMinter {
		return fmt.Errorf("%v is not minter", routerPDA)
	}
	return nil
}

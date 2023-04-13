package flow

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
	b.CrossChainBridgeBase.InitAfterConfig()
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
	if routerContract == "" {
		return nil
	}

	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	routerMPC, err := b.GetMPCAddress()
	if err != nil {
		log.Warn("get router mpc address failed", "routerContract", routerContract, "err", err)
		return err
	}
	if routerMPC == "" {
		log.Warn("get router mpc address return an empty address", "routerContract", routerContract)
		return fmt.Errorf("empty router mpc address")
	}
	log.Info("get router mpc address success", "routerContract", routerContract, "routerMPC", routerMPC)
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Warn("get mpc public key failed", "mpc", routerMPC, "err", err)
		return err
	}
	if err = b.VerifyPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Warn("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
		return err
	}
	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC: routerMPC,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID), "routerContract", routerContract, "routerMPC", routerMPC)
	if mongodb.HasClient() {
		nextSwapNonce, err := mongodb.FindNextSwapNonce(chainID, strings.ToLower(routerMPC))
		if err == nil {
			log.Info("init next swap nonce from db", "chainID", chainID, "mpc", routerMPC, "nonce", nextSwapNonce)
			b.InitSwapNonce(b, routerMPC, nextSwapNonce)
		}
	}

	return nil
}

// SetTokenConfig set and verify token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)

	if tokenCfg == nil || !tokens.IsERC20Router() {
		return
	}

	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(isReload, log.Error, log.Fatal)

	tokenID := tokenCfg.TokenID

	decimals, errt := b.GetTokenDecimals(tokenAddr)
	if errt != nil {
		logErrFunc("get token decimals failed", "tokenID", tokenID, "tokenAddr", tokenAddr, "err", errt)
		if isReload {
			return
		}
	}
	if decimals != tokenCfg.Decimals {
		logErrFunc("token decimals mismatch", "tokenID", tokenID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
		if isReload {
			return
		}
	}
	underlying, err := b.GetUnderlyingAddress(tokenAddr)
	if err != nil && tokenCfg.IsStandardTokenVersion() {
		logErrFunc("get underlying address failed", "tokenID", tokenID, "tokenAddr", tokenAddr, "err", err)
		if isReload {
			return
		}
	}
	tokenCfg.SetUnderlying(underlying) // init underlying address
}

// GetTokenDecimals query token decimals
func (b *Bridge) GetTokenDecimals(tokenAddr string) (uint8, error) {
	return 8, nil
}

// GetUnderlyingAddress query underlying address
func (b *Bridge) GetUnderlyingAddress(contractAddr string) (string, error) {
	return "", nil
}

// GetMPCAddress query mpc address
func (b *Bridge) GetMPCAddress() (string, error) {
	return b.GetChainConfig().Extra, nil
}

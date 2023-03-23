package iota

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// SetGatewayConfig set gateway config
func (b *Bridge) SetGatewayConfig(gatewayCfg *tokens.GatewayConfig) {
	b.CrossChainBridgeBase.SetGatewayConfig(gatewayCfg)
}

// SetTokenConfig set token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)
	if tokenCfg == nil {
		return
	}

	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)

	tokenID := tokenCfg.TokenID

	err := b.VerifyTokenConfig(tokenCfg)
	if err != nil {
		logErrFunc("verify token config failed", "chainID", b.ChainConfig.ChainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "err", err)
		return
	}
	log.Info("verify token config success", "chainID", b.ChainConfig.ChainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals)
}

// VerifyTokenConfig verify token config
func (b *Bridge) VerifyTokenConfig(tokenCfg *tokens.TokenConfig) error {
	return nil
}

// InitRouterInfo init router info (in ripple routerContract is routerMPC)
func (b *Bridge) InitRouterInfo(routerContract string, routerVersion string) (err error) {
	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	routerMPC := routerContract // in ripple routerMPC is routerContract
	if !b.IsValidAddress(routerMPC) {
		log.Warn("wrong router mpc address (in ripple routerMPC is routerContract)", "routerMPC", routerMPC)
		return fmt.Errorf("wrong router mpc address: %v", routerMPC)
	}
	log.Info("get router mpc address success", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
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
		chainID,
		&router.SwapRouterInfo{
			RouterMPC: routerMPC,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID),
		"routerContract", routerContract, "routerMPC", routerMPC)

	if mongodb.HasClient() {
		var nextSwapNonce uint64
		for i := 0; i < 3; i++ {
			nextSwapNonce, err = mongodb.FindNextSwapNonce(chainID, routerMPC)
			if err == nil {
				break
			}
		}
		b.InitSwapNonce(b, routerMPC, nextSwapNonce)
	}

	return nil
}

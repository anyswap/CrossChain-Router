package btc

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	fixDecimals uint8 = 8
)

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
	if routerContract == "" {
		return nil
	}

	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	routerMPC := routerContract // in btc routerMPC is routerContract
	if !b.IsValidAddress(routerMPC) {
		log.Warn("wrong router mpc address (in ripple routerMPC is routerContract)", "routerMPC", routerMPC)
		return fmt.Errorf("wrong router mpc address: %v", routerMPC)
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

	if fixDecimals != tokenCfg.Decimals {
		logErrFunc("token decimals mismatch", "tokenID", tokenID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "fixDecimals", fixDecimals)
		if isReload {
			return
		}
	}
}

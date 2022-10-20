package near

import (
	"encoding/json"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	GetFtMetadata = "ft_metadata"
	GetFtBalance  = "ft_balance_of"
	EmptyArgs     = "e30="
)

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract string) (err error) {
	if routerContract == "" {
		return nil
	}

	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	routerMPC := b.GetRouterContract("")
	if routerMPC == "" {
		log.Warn("get router mpc address return an empty address", "routerContract", routerContract)
		return fmt.Errorf("empty router mpc address")
	}
	log.Info("get router mpc address success", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
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
}

// GetTokenDecimals query token decimals
func (b *Bridge) GetTokenDecimals(tokenAddr string) (uint8, error) {
	if tokenAddr == b.GetRouterContract("") {
		return uint8(24), nil
	}
	urls := append(b.GatewayConfig.APIAddress, b.GatewayConfig.APIAddressExt...)
	for _, url := range urls {
		result, err := functionCall(url, tokenAddr, GetFtMetadata, EmptyArgs)
		if err == nil {
			ftMetadata := &FungibleTokenMetadata{}
			errf := json.Unmarshal(result, ftMetadata)
			if errf != nil {
				return 0, errf
			}
			return ftMetadata.Decimals, nil
		}
	}
	return 0, tokens.ErrTokenDecimals
}

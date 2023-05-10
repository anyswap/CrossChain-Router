package ripple

import (
	"fmt"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
)

var (
	currencyMap = new(sync.Map)
	issuerMap   = new(sync.Map)
	assetMap    = new(sync.Map)
)

// ripple token address format is "XRP" or "Currency/Issuser"
func convertToAsset(tokenAddr string) (*data.Asset, error) {
	return data.NewAsset(tokenAddr)
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
	asset, err := convertToAsset(tokenCfg.ContractAddress)
	if err != nil {
		return err
	}
	currency, err := data.NewCurrency(asset.Currency)
	if err != nil {
		return fmt.Errorf("invalid currency '%v', %w", asset.Currency, err)
	}
	currencyMap.Store(asset.Currency, &currency)
	configedDecimals := tokenCfg.Decimals
	if currency.IsNative() {
		if configedDecimals != 6 {
			return fmt.Errorf("invalid native decimals: want 6 but have %v", configedDecimals)
		}
		if asset.Issuer != "" {
			return fmt.Errorf("native currency should not have issuer")
		}
	} else {
		if asset.Issuer == "" {
			return fmt.Errorf("non native currency must have issuer")
		}
		issuer, errf := data.NewAccountFromAddress(asset.Issuer)
		if errf != nil {
			return fmt.Errorf("invalid issuer '%v', %w", asset.Issuer, errf)
		}
		issuerMap.Store(asset.Issuer, issuer)
	}
	assetMap.Store(tokenCfg.ContractAddress, asset)
	return nil
}

// InitRouterInfo init router info (in ripple routerContract is routerMPC)
func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
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
	if routerMPCPubkey == "empty" {
		log.Info("ignore configed empty public key", "routerMPC", routerMPC)
	} else if err = VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
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

	return nil
}

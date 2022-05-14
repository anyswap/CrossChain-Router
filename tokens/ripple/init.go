package ripple

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/websockets"
)

var (
	currencyMap = make(map[string]*data.Currency)
	issuerMap   = make(map[string]*data.Account)
	assetMap    = make(map[string]*data.Asset)
)

// ripple token address format is "XRP" or "Currency/Issuser"
func convertToAsset(tokenAddr string) (*data.Asset, error) {
	return data.NewAsset(tokenAddr)
}

// SetGatewayConfig set gateway config
func (b *Bridge) SetGatewayConfig(gatewayCfg *tokens.GatewayConfig) {
	b.CrossChainBridgeBase.SetGatewayConfig(gatewayCfg)
	b.InitRemotes()
}

// InitRemotes set ripple remotes
func (b *Bridge) InitRemotes() {
	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)
	b.Remotes = make(map[string]*websockets.Remote)
	for _, apiAddress := range b.GetGatewayConfig().APIAddress {
		remote, err := websockets.NewRemote(apiAddress)
		if err != nil || remote == nil {
			log.Warn("Cannot connect to ripple", "address", apiAddress, "error", err)
			continue
		}
		log.Info("Connected to remote api success", "api", apiAddress)
		b.Remotes[apiAddress] = remote
	}
	if len(b.Remotes) < 1 {
		logErrFunc("No available remote api")
		return
	}
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
	currencyMap[asset.Currency] = &currency
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
		issuerMap[asset.Issuer] = issuer
	}
	assetMap[tokenCfg.ContractAddress] = asset
	return nil
}

// InitRouterInfo init router info (in ripple routerContract is routerMPC)
func (b *Bridge) InitRouterInfo(routerContract string) (err error) {
	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	routerMPC := routerContract // in ripple routerMPC is routerContract
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
	if err = VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Warn("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
		return err
	}
	router.SetRouterInfo(
		routerContract,
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
			nextSwapNonce, err = mongodb.FindNextSwapNonce(chainID, strings.ToLower(routerMPC))
			if err == nil {
				break
			}
		}
		b.InitSwapNonce(b, routerMPC, nextSwapNonce)
	}

	return nil
}

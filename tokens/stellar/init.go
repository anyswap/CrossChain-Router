package stellar

import (
	"fmt"
	"strings"
	"sync"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/protocols/horizon/operations"
	"github.com/stellar/go/txnbuild"
)

var (
	currencyMap = new(sync.Map)
	assetMap    = new(sync.Map)
)

// stellar address format is "NATIVE" or "code/issuer"
func convertToAsset(tokenID, tokenAddr string) (txnbuild.BasicAsset, error) {
	if tokenAddr == "" {
		return nil, fmt.Errorf("non native asset must have issuer")
	}
	if strings.EqualFold(tokenAddr, "native") {
		return txnbuild.NativeAsset{}, nil
	}
	token := strings.Split(tokenAddr, "/")
	switch len(token) {
	case 2:
		if token[0] != tokenID {
			return nil, fmt.Errorf("asset code must equal")
		}
		return txnbuild.CreditAsset{Code: token[0], Issuer: token[1]}, nil
	default:
		return nil, fmt.Errorf("non native asset must have issuer")
	}
}

func convertTokenID(payment *operations.Payment) string {
	if isNativeAsset(payment.Asset.Type) {
		return "XLM"
	}
	return payment.Asset.Code + "/" + payment.Asset.Issuer
}

// SetGatewayConfig set gateway config
func (b *Bridge) SetGatewayConfig(gatewayCfg *tokens.GatewayConfig) {
	b.CrossChainBridgeBase.SetGatewayConfig(gatewayCfg)
	b.InitRemotes()
}

// InitRemotes set stellar remotes
func (b *Bridge) InitRemotes() {
	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)
	remotes := make(map[string]*horizonclient.Client)
	for _, apiAddress := range b.GetGatewayConfig().AllGatewayURLs {
		remote := horizonclient.DefaultPublicNetClient
		remote.HorizonURL = apiAddress
		log.Info("Connected to remote api success", "api", apiAddress)
		remotes[apiAddress] = remote
	}
	if len(remotes) < 1 {
		logErrFunc("No available remote api")
		return
	}
	b.Remotes = remotes
}

// SetTokenConfig set token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
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

	asset, _ := convertToAsset(tokenCfg.TokenID, tokenCfg.ContractAddress)
	if asset.IsNative() {
		b.CrossChainBridgeBase.SetTokenConfig(tokenCfg.TokenID, tokenCfg)
		b.CrossChainBridgeBase.SetTokenConfig("native", tokenCfg)
	} else {
		b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)
	}
	log.Info("verify token config success", "chainID", b.ChainConfig.ChainID, "tokenID", tokenID, "tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals)
}

// VerifyTokenConfig verify token config
func (b *Bridge) VerifyTokenConfig(tokenCfg *tokens.TokenConfig) error {
	asset, err := convertToAsset(tokenCfg.TokenID, tokenCfg.ContractAddress)
	if err != nil {
		return err
	}
	if !asset.IsNative() {
		assetStat, err1 := b.GetAsset(asset.GetCode(), asset.GetIssuer())
		if err1 != nil {
			return err1
		}
		currencyMap.Store(tokenCfg.ContractAddress, assetStat)
	}
	// TokenID format is code:issuer or native
	assetMap.Store(tokenCfg.ContractAddress, asset)
	return nil
}

// InitRouterInfo init router info (in stellar routerContract is routerMPC)
func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	routerMPC := routerContract // in stellar routerMPC is routerContract
	if !b.IsValidAddress(routerMPC) {
		log.Warn("wrong router mpc address (in stellar routerMPC is routerContract)", "routerMPC", routerMPC)
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

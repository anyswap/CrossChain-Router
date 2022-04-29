// Package bridge init router bridge and load / reload configs.
package bridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// InitRouterBridges init router bridges
//nolint:funlen,gocyclo // ok
func InitRouterBridges(isServer bool) {
	log.Info("start init router bridges")
	router.IsIniting = true
	defer func() {
		router.IsIniting = false
	}()

	dontPanic := params.GetExtraConfig().DontPanicInInitRouter
	logErrFunc := log.GetLogFuncOr(dontPanic, log.Error, log.Fatal)

	client.InitHTTPClient()
	router.InitRouterConfigClients()

	allChainIDs, err := router.GetAllChainIDs()
	if err != nil {
		logErrFunc("call GetAllChainIDs failed", "err", err)
	}
	// get rid of blacked chainIDs
	chainIDs := make([]*big.Int, 0, len(allChainIDs))
	for _, chainID := range allChainIDs {
		if params.IsChainIDInBlackList(chainID.String()) {
			log.Debugf("ingore chainID %v in black list", chainID)
			continue
		}
		chainIDs = append(chainIDs, chainID)
	}
	router.AllChainIDs = chainIDs
	log.Info("get all chain ids success", "chainIDs", chainIDs)
	if len(router.AllChainIDs) == 0 {
		logErrFunc("empty chain IDs")
	}

	allTokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		logErrFunc("call GetAllTokenIDs failed", "err", err)
	}
	// get rid of blacked tokenIDs
	tokenIDs := make([]string, 0, len(allTokenIDs))
	for _, tokenID := range allTokenIDs {
		if params.IsTokenIDInBlackList(tokenID) {
			log.Debugf("ingore tokenID %v in black list", tokenID)
			continue
		}
		tokenIDs = append(tokenIDs, tokenID)
	}
	router.AllTokenIDs = tokenIDs
	log.Info("get all token ids success", "tokenIDs", tokenIDs)
	if len(router.AllTokenIDs) == 0 && !tokens.IsAnyCallRouter() {
		logErrFunc("empty token IDs")
	}

	for _, chainID := range chainIDs {
		bridge := NewCrossChainBridge(chainID)
		configLoader, ok := bridge.(tokens.IBridgeConfigLoader)
		if !ok {
			logErrFunc("do not support onchain config loading", "chainID", chainID)
			if dontPanic {
				continue
			}
		}

		configLoader.InitGatewayConfig(chainID, false)
		AdjustGatewayOrder(bridge, chainID.String())
		configLoader.InitChainConfig(chainID, false)

		bridge.InitAfterConfig(false)
		router.RouterBridges[chainID.String()] = bridge

		for _, tokenID := range tokenIDs {
			configLoader.InitTokenConfig(tokenID, chainID, false)
		}
	}
	router.PrintMultichainTokens()

	err = loadSwapConfigs()
	if err != nil {
		logErrFunc("load swap configs failed", "err", err)
	}

	if params.SignWithPrivateKey() {
		for _, chainID := range chainIDs {
			priKey := params.GetSignerPrivateKey(chainID.String())
			if priKey == "" {
				logErrFunc("missing config private key", "chainID", chainID)
			}
		}
	} else {
		mpc.Init(params.GetMPCConfig(), isServer)
	}

	log.Info("init router bridges success", "isServer", isServer)
}

func loadSwapConfigs() error {
	if !tokens.IsERC20Router() {
		return nil
	}
	swapConfigs := make(map[string]map[string]*tokens.SwapConfig)
	for _, tokenID := range router.AllTokenIDs {
		swapConfigs[tokenID] = make(map[string]*tokens.SwapConfig)
		for _, chainID := range router.AllChainIDs {
			multichainToken := router.GetCachedMultichainToken(tokenID, chainID.String())
			if multichainToken == "" {
				log.Debug("ignore swap config as no multichain token exist", "tokenID", tokenID, "chainID", chainID)
				continue
			}
			swapCfg, err := router.GetSwapConfig(tokenID, chainID)
			if err != nil {
				log.Warn("get swap config failed", "tokenID", tokenID, "chainID", chainID, "err", err)
				return err
			}
			err = swapCfg.CheckConfig()
			if err != nil {
				log.Warn("check swap config failed", "tokenID", tokenID, "chainID", chainID, "err", err)
				return err
			}
			swapConfigs[tokenID][chainID.String()] = swapCfg
			log.Info("load swap config success", "tokenID", tokenID, "chainID", chainID, "multichainToken", multichainToken)
		}
	}
	tokens.SetSwapConfigs(swapConfigs)
	log.Info("load all swap config success")
	return nil
}

// Package bridge init router bridge and load / reload configs.
package bridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
)

// NewCrossChainBridge new bridge
func NewCrossChainBridge(*big.Int) tokens.IBridge {
	return eth.NewCrossChainBridge()
}

// InitRouterBridges init router bridges
func InitRouterBridges(isServer bool) {
	log.Info("start init router bridges")

	router.InitRouterConfigClients()

	chainIDs, err := router.GetAllChainIDs()
	if err != nil {
		log.Fatal("call GetAllChainIDs failed", "err", err)
	}
	router.AllChainIDs = chainIDs
	log.Info("get all chain ids success", "chainIDs", chainIDs)

	tokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		log.Fatal("call GetAllTokenIDs failed", "err", err)
	}
	router.AllTokenIDs = tokenIDs
	log.Info("get all token ids success", "tokenIDs", tokenIDs)

	for _, chainID := range chainIDs {
		if params.IsChainIDInBlackList(chainID.String()) {
			log.Warnf("ingore chainID %v in black list", chainID)
			continue
		}
		bridge := NewCrossChainBridge(chainID)

		bridge.InitGatewayConfig(chainID)
		AdjustGatewayOrder(bridge, chainID.String())
		bridge.InitChainConfig(chainID)

		for _, tokenID := range tokenIDs {
			bridge.InitTokenConfig(tokenID, chainID)
		}

		router.RouterBridges[chainID.String()] = bridge
	}
	router.PrintMultichainTokens()

	err = loadSwapConfigs()
	if err != nil {
		log.Fatal("load swap configs failed", "err", err)
	}

	cfg := params.GetRouterConfig()
	mpc.Init(cfg.MPC, isServer)

	startReloadRouterConfigTask()

	log.Info(">>> init router bridges success", "isServer", isServer)
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

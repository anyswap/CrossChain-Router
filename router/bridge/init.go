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
func InitRouterBridges(isServer bool) {
	log.Info("start init router bridges")
	router.IsIniting = true

	client.InitHTTPClient()
	router.InitRouterConfigClients()

	allChainIDs, err := router.GetAllChainIDs()
	if err != nil {
		log.Fatal("call GetAllChainIDs failed", "err", err)
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
		log.Fatal("empty chain IDs")
	}

	allTokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		log.Fatal("call GetAllTokenIDs failed", "err", err)
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
		log.Fatal("empty token IDs")
	}

	for _, chainID := range chainIDs {
		bridge := NewCrossChainBridge(chainID)
		configLoader, ok := bridge.(tokens.IBridgeConfigLoader)
		if !ok {
			log.Fatal("do not support onchain config loading", "chainID", chainID)
		}

		configLoader.InitGatewayConfig(chainID)
		AdjustGatewayOrder(bridge, chainID.String())
		configLoader.InitChainConfig(chainID)

		for _, tokenID := range tokenIDs {
			configLoader.InitTokenConfig(tokenID, chainID)
		}

		bridge.InitAfterConfig()

		router.RouterBridges[chainID.String()] = bridge
	}
	router.PrintMultichainTokens()

	err = loadSwapAndFeeConfigs()
	if err != nil {
		log.Fatal("load swap and fee configs failed", "err", err)
	}

	if params.SignWithPrivateKey() {
		for _, chainID := range chainIDs {
			priKey := params.GetSignerPrivateKey(chainID.String())
			if priKey == "" {
				log.Fatalf("missing config private key on chain %v", chainID)
			}
		}
	} else {
		mpc.Init(params.GetMPCConfig(), isServer)
	}

	startReloadRouterConfigTask()

	log.Info("init router bridges success", "isServer", isServer)

	router.IsIniting = false
}

func loadSwapAndFeeConfigs() (err error) {
	if !tokens.IsERC20Router() {
		return nil
	}
	for _, tokenID := range router.AllTokenIDs {
		supportChainIDs := make([]*big.Int, 0, len(router.AllChainIDs))
		for _, chainID := range router.AllChainIDs {
			multichainToken := router.GetCachedMultichainToken(tokenID, chainID.String())
			if multichainToken != "" {
				supportChainIDs = append(supportChainIDs, chainID)
			}
		}
		if err = loadSwapConfigs(supportChainIDs); err != nil {
			return err
		}
		if err = loadFeeConfigs(supportChainIDs); err != nil {
			return err
		}
	}
	return nil
}

func loadSwapConfigs(supportChainIDs []*big.Int) error {
	swapConfigs := make(map[string]map[string]map[string]*tokens.SwapConfig)

	for _, tokenID := range router.AllTokenIDs {
		tmap := make(map[string]map[string]*tokens.SwapConfig)
		swapConfigs[tokenID] = tmap
		for i, fromChainID := range supportChainIDs {
			fmap := make(map[string]*tokens.SwapConfig)
			tmap[fromChainID.String()] = fmap
			for j, toChainID := range supportChainIDs {
				if i == j {
					continue
				}
				swapCfg, err := router.GetActualSwapConfig(tokenID, fromChainID, toChainID)
				if err != nil {
					log.Warn("get swap config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return err
				}
				err = swapCfg.CheckConfig()
				if err != nil {
					log.Warn("check swap config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return err
				}
				fmap[toChainID.String()] = swapCfg
				log.Info("load swap config success", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID)
			}
		}
	}

	tokens.SetSwapConfigs(swapConfigs)
	log.Info("load all swap config success")
	return nil
}

func loadFeeConfigs(supportChainIDs []*big.Int) error {
	feeConfigs := make(map[string]map[string]map[string]*tokens.FeeConfig)

	for _, tokenID := range router.AllTokenIDs {
		tmap := make(map[string]map[string]*tokens.FeeConfig)
		feeConfigs[tokenID] = tmap
		for i, fromChainID := range supportChainIDs {
			fmap := make(map[string]*tokens.FeeConfig)
			tmap[fromChainID.String()] = fmap
			for j, toChainID := range supportChainIDs {
				if i == j {
					continue
				}
				feeCfg, err := router.GetActualFeeConfig(tokenID, fromChainID, toChainID)
				if err != nil {
					log.Warn("get fee config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return err
				}
				err = feeCfg.CheckConfig()
				if err != nil {
					log.Warn("check fee config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return err
				}
				fmap[toChainID.String()] = feeCfg
				log.Info("load fee config success", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID)
			}
		}
	}

	tokens.SetFeeConfigs(feeConfigs)
	log.Info("load all fee config success")
	return nil
}

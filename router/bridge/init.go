// Package bridge init router bridge and load / reload configs.
package bridge

import (
	"math/big"
	"sync"

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
	log.Info("start init router bridges", "isServer", isServer)
	var success bool
	router.IsIniting = true
	defer func() {
		router.IsIniting = false
		log.Info("init router bridges finished", "isServer", isServer, "success", success)
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

	wg := new(sync.WaitGroup)
	wg.Add(len(chainIDs))
	for _, chainID := range chainIDs {
		go func(wg *sync.WaitGroup, chainID *big.Int) {
			defer wg.Done()

			bridge := NewCrossChainBridge(chainID)
			configLoader, ok := bridge.(tokens.IBridgeConfigLoader)
			if !ok {
				logErrFunc("do not support onchain config loading", "chainID", chainID)
				if dontPanic {
					return
				}
			}

			configLoader.InitGatewayConfig(chainID, false)
			AdjustGatewayOrder(bridge, chainID.String())
			configLoader.InitChainConfig(chainID, false)

			bridge.InitAfterConfig(false)
			router.SetBridge(chainID.String(), bridge)

			wg2 := new(sync.WaitGroup)
			wg2.Add(len(tokenIDs))
			for _, tokenID := range tokenIDs {
				go func(wg2 *sync.WaitGroup, tokenID string, chainID *big.Int) {
					defer wg2.Done()
					log.Info("start load token config", "tokenID", tokenID, "chainID", chainID)
					configLoader.InitTokenConfig(tokenID, chainID, false)
				}(wg2, tokenID, chainID)
			}
			wg2.Wait()
		}(wg, chainID)
	}
	wg.Wait()

	router.PrintMultichainTokens()

	loadSwapConfigs(dontPanic)

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

	success = true
}

func loadSwapConfigs(dontPanic bool) {
	if !tokens.IsERC20Router() {
		return
	}
	logErrFunc := log.GetLogFuncOr(dontPanic, log.Error, log.Fatal)
	swapConfigs := new(sync.Map)
	wg := new(sync.WaitGroup)
	for _, tokenID := range router.AllTokenIDs {
		swapConfig := new(sync.Map)
		swapConfigs.Store(tokenID, swapConfig)
		for _, chainID := range router.AllChainIDs {
			wg.Add(1)
			go func(wg *sync.WaitGroup, tokenID string, chainID *big.Int) {
				defer wg.Done()

				multichainToken := router.GetCachedMultichainToken(tokenID, chainID.String())
				if multichainToken == "" {
					log.Debug("ignore swap config as no multichain token exist", "tokenID", tokenID, "chainID", chainID)
					return
				}
				swapCfg, err := router.GetSwapConfig(tokenID, chainID)
				if err != nil {
					logErrFunc("get swap config failed", "tokenID", tokenID, "chainID", chainID, "err", err)
					return
				}
				err = swapCfg.CheckConfig()
				if err != nil {
					logErrFunc("check swap config failed", "tokenID", tokenID, "chainID", chainID, "err", err)
					return
				}
				swapConfig.Store(chainID.String(), swapCfg)
				log.Info("load swap config success", "tokenID", tokenID, "chainID", chainID, "multichainToken", multichainToken)
			}(wg, tokenID, chainID)
		}
	}
	wg.Wait()
	tokens.SetSwapConfigs(swapConfigs)
	log.Info("load all swap config success")
}

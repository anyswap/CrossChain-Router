package bridge

import (
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var reloadRouterConfigLock sync.Mutex

// StartReloadRouterConfigTask start reload config
func StartReloadRouterConfigTask() {
	// method 1: use web socket event subscriber
	go router.SubscribeUpdateConfig(ReloadRouterConfig)

	// method 2: use fix period timer
	go doReloadRouterConfigPeriodly()

	// method 3: trigger manually by signals
	go doReloadRouterConfigManually()
}

func doReloadRouterConfigPeriodly() {
	reloadCycle := params.GetRouterConfig().Onchain.ReloadCycle
	if reloadCycle == 0 {
		return
	}
	log.Info("start reload router config task periodly", "reloadCycle", reloadCycle)
	reloadInterval := time.Duration(reloadCycle) * time.Second
	reloadTimer := time.NewTimer(reloadInterval)
	for {
		<-reloadTimer.C
		reloadTimer.Reset(reloadInterval)
		for i := 0; i < 3; i++ {
			success := ReloadRouterConfig()
			if success {
				break
			}
		}
	}
}

func doReloadRouterConfigManually() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGUSR1)
	isReloading := false
	for {
		sig := <-signalChan
		if isReloading {
			log.Info("ignore signal to reload router config in reloading", "signal", sig)
			continue
		}
		log.Info("receive signal to reload router config", "signal", sig)
		isReloading = true
		go func() {
			ReloadRouterConfig()
			isReloading = false
		}()
	}
}

// ReloadRouterConfig reload router config
// support add/remove/modify chain config
// support add/remove/modify token config
//nolint:funlen,gocyclo // ok
func ReloadRouterConfig() bool {
	log.Info("[reload] start reload router config")
	reloadRouterConfigLock.Lock()
	router.IsIniting = true
	defer func() {
		router.IsIniting = false
		reloadRouterConfigLock.Unlock()
	}()

	// reload local config
	params.ReloadRouterConfig()

	allChainIDs, err := router.GetAllChainIDs()
	if err != nil {
		log.Error("[reload] call GetAllChainIDs failed", "err", err)
		return false
	}

	// get rid of blacked chainIDs
	chainIDs := make([]*big.Int, 0, len(allChainIDs))
	for _, chainID := range allChainIDs {
		if params.IsChainIDInBlackList(chainID.String()) {
			log.Debugf("[reload] ingore chainID %v in black list", chainID)
			continue
		}
		chainIDs = append(chainIDs, chainID)
	}
	log.Info("[reload] get all chain ids success", "chainIDs", chainIDs)
	if len(chainIDs) == 0 {
		log.Error("[reload] empty chain IDs")
	}

	// get rid of removed bridges
	for _, chainID := range router.AllChainIDs {
		exist := false
		for _, newChainID := range chainIDs {
			if chainID.Cmp(newChainID) == 0 {
				exist = true
				break
			}
		}
		if !exist {
			router.RouterBridges[chainID.String()] = nil
		}
	}

	// update current chainIDs
	router.AllChainIDs = chainIDs

	allTokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		log.Error("[reload] call GetAllTokenIDs failed", "err", err)
		return false
	}

	// get rid of blacked tokenIDs
	tokenIDs := make([]string, 0, len(allTokenIDs))
	for _, tokenID := range allTokenIDs {
		if params.IsTokenIDInBlackList(tokenID) {
			log.Debugf("[reload] ingore tokenID %v in black list", tokenID)
			continue
		}
		tokenIDs = append(tokenIDs, tokenID)
	}
	log.Info("[reload] get all token ids success", "tokenIDs", tokenIDs)
	if len(tokenIDs) == 0 && !tokens.IsAnyCallRouter() {
		log.Error("[reload] empty token IDs")
	}

	removedTokenIDs := make([]string, 0)
	for _, tokenID := range router.AllTokenIDs {
		exist := false
		for _, newTokenID := range tokenIDs {
			if tokenID == newTokenID {
				exist = true
				break
			}
		}
		if !exist {
			removedTokenIDs = append(removedTokenIDs, tokenID)
		}
	}
	if len(removedTokenIDs) > 0 {
		log.Info("[reload] remove token ids", "removedTokenIDs", removedTokenIDs)
	}

	// update current tokenIDs
	router.AllTokenIDs = tokenIDs

	for _, chainID := range chainIDs {
		chainIDStr := chainID.String()
		bridge := router.GetBridgeByChainID(chainIDStr)
		isNewBridge := false
		if bridge == nil {
			log.Info("[reload] add new bridge", "chainID", chainID)
			bridge = NewCrossChainBridge(chainID)
			isNewBridge = true
		}
		configLoader, ok := bridge.(tokens.IBridgeConfigLoader)
		if !ok {
			log.Warn("[reload] do not support onchain config reloading", "chainID", chainID)
			continue
		}

		log.Info("[reload] set chain config", "chainID", chainID)
		configLoader.InitGatewayConfig(chainID, true)
		AdjustGatewayOrder(bridge, chainID.String())
		configLoader.InitChainConfig(chainID, true)

		if isNewBridge {
			bridge.InitAfterConfig(true)
			router.RouterBridges[chainIDStr] = bridge
		}

		for _, tokenID := range removedTokenIDs {
			tokenAddr := router.GetCachedMultichainToken(tokenID, chainIDStr)
			router.MultichainTokens[strings.ToLower(tokenID)] = nil

			if tokenAddr != "" {
				log.Info("[reload] remove token config", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr)
				configLoader.RemoveTokenConfig(tokenAddr)
			}
		}

		for _, tokenID := range tokenIDs {
			log.Info("[reload] set token config", "tokenID", tokenID, "chainID", chainID)
			configLoader.InitTokenConfig(tokenID, chainID, true)
		}
	}

	err = loadSwapConfigs()
	if err != nil {
		log.Error("[reload] load swap configs failed", "err", err)
		return false
	}

	log.Info("[reload] reload router config success")
	return true
}

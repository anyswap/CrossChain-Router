package bridge

import (
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
)

var reloadRouterConfigLock sync.Mutex

func startReloadRouterConfigTask() {
	log.Info("start reload router config task")
	go doReloadRouterConfigTask()
}

func doReloadRouterConfigTask() {
	// method 1: use web socket event subscriber
	router.SubscribeUpdateConfig(ReloadRouterConfig)

	// method 2: use fix period timer
	reloadCycle := params.GetRouterConfig().Onchain.ReloadCycle
	if reloadCycle == 0 {
		log.Info("stop reload router config task as it's disabled")
		return
	}
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

// ReloadRouterConfig reload router config
// support modify exist chain config
// support add/remove/modify token config
func ReloadRouterConfig() bool {
	log.Info("[reload] start reload router config")
	reloadRouterConfigLock.Lock()
	defer reloadRouterConfigLock.Unlock()

	chainIDs := router.AllChainIDs
	log.Info("[reload] get all chain ids success", "chainIDs", chainIDs)

	tokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		log.Error("[reload] call GetAllTokenIDs failed", "err", err)
		return false
	}
	log.Info("[reload] get all token ids success", "tokenIDs", tokenIDs)

	removedTokenIDs := make([]string, 0)
	for _, tokenID := range router.AllTokenIDs {
		exist := false
		for _, newTokenIDs := range tokenIDs {
			if tokenID == newTokenIDs {
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
	router.AllTokenIDs = tokenIDs

	for _, chainID := range chainIDs {
		chainIDStr := chainID.String()
		bridge := router.GetBridgeByChainID(chainIDStr)
		if bridge == nil {
			log.Warn("[reload] do not support new chainID", "chainID", chainID)
			continue
		}

		log.Info("[reload] set chain config", "chainID", chainID)
		bridge.ReloadChainConfig(chainID)

		for _, tokenID := range removedTokenIDs {
			tokenAddr := router.GetCachedMultichainToken(tokenID, chainIDStr)
			if tokenAddr != "" {
				log.Info("[reload] remove token config", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr)
				bridge.RemoveTokenConfig(tokenAddr)
			}
			router.MultichainTokens[strings.ToLower(tokenID)] = nil
		}

		for _, tokenID := range tokenIDs {
			log.Info("[reload] set token config", "tokenID", tokenID, "chainID", chainID)
			bridge.ReloadTokenConfig(tokenID, chainID)
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

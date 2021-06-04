package bridge

import (
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
)

// ReloadRouterConfig reload router config
// support modify exist chain config
// support add/remove/modify token config
func ReloadRouterConfig() {
	chainIDs := router.AllChainIDs
	log.Info("[reload] get all chain ids success", "chainIDs", chainIDs)

	tokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		log.Fatal("[reload] call GetAllTokenIDs failed", "err", err)
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
			log.Error("[reload] do not support new chainID", "chainID", chainID)
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
}

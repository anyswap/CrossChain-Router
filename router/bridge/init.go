package bridge

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/mpc"
	"github.com/anyswap/CrossChain-Router/params"
	"github.com/anyswap/CrossChain-Router/router"
	"github.com/anyswap/CrossChain-Router/tokens"
	"github.com/anyswap/CrossChain-Router/tokens/eth"
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
		bridge := NewCrossChainBridge(chainID)

		bridge.InitGatewayConfig(chainID)
		bridge.InitChainConfig(chainID)

		for _, tokenID := range tokenIDs {
			bridge.InitTokenConfig(tokenID, chainID)
		}

		router.RouterBridges[chainID.String()] = bridge
	}
	router.PrintMultichainTokens()

	cfg := params.GetRouterConfig()
	mpc.Init(cfg.MPC, isServer)

	router.SubscribeUpdateConfig(ReloadRouterConfig)

	log.Info(">>> init router bridges success", "isServer", isServer)
}

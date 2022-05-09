// Package bridge init router bridge and load / reload configs.
package bridge

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	routerInfoIsLoaded = new(sync.Map) // key is router contract address
)

func isRouterInfoLoaded(routerContract string) bool {
	_, exist := routerInfoIsLoaded.Load(routerContract)
	return exist
}

// InitRouterBridges init router bridges
//nolint:funlen,gocyclo // ok
func InitRouterBridges(isServer bool) {
	log.Info("start init router bridges", "isServer", isServer)
	var success bool
	router.IsIniting = true
	defer func() {
		router.IsIniting = false
		routerInfoIsLoaded = new(sync.Map)
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
	log.Info("get all chain ids success", "chainIDs", chainIDs)
	if len(chainIDs) == 0 {
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
	log.Info("get all token ids success", "tokenIDs", tokenIDs)
	if len(tokenIDs) == 0 && !tokens.IsAnyCallRouter() {
		logErrFunc("empty token IDs")
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(chainIDs))
	for _, chainID := range chainIDs {
		go func(wg *sync.WaitGroup, chainID *big.Int) {
			defer wg.Done()

			bridge := NewCrossChainBridge(chainID)

			InitGatewayConfig(bridge, chainID)
			AdjustGatewayOrder(bridge, chainID.String())
			InitChainConfig(bridge, chainID)

			bridge.InitAfterConfig()
			router.SetBridge(chainID.String(), bridge)

			wg2 := new(sync.WaitGroup)
			wg2.Add(len(tokenIDs))
			for _, tokenID := range tokenIDs {
				go func(wg2 *sync.WaitGroup, tokenID string, chainID *big.Int) {
					defer wg2.Done()
					log.Info("start load token config", "tokenID", tokenID, "chainID", chainID)
					InitTokenConfig(bridge, tokenID, chainID)
				}(wg2, tokenID, chainID)
			}
			wg2.Wait()
		}(wg, chainID)
	}
	wg.Wait()

	router.AllChainIDs = chainIDs
	router.AllTokenIDs = tokenIDs

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

// InitGatewayConfig impl
func InitGatewayConfig(b tokens.IBridge, chainID *big.Int) {
	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(isReload, log.Error, log.Fatal)
	if chainID == nil || chainID.Sign() == 0 {
		logErrFunc("init gateway with zero chain ID")
		if isReload {
			return
		}
	}
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		logErrFunc("gateway not found for chain ID", "chainID", chainID)
		if isReload {
			return
		}
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	b.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	if !isReload {
		latestBlock, err := b.GetLatestBlockNumber()
		if err != nil && router.IsIniting {
			for i := 0; i < router.RetryRPCCountInInit; i++ {
				if latestBlock, err = b.GetLatestBlockNumber(); err == nil {
					break
				}
				time.Sleep(router.RetryRPCIntervalInInit)
			}
		}
		if err != nil {
			logErrFunc("get lastest block number failed", "chainID", chainID, "err", err)
			if isReload {
				return
			}
		}
		log.Infof("[%5v] lastest block number is %v", chainID, latestBlock)
	}
	log.Info(fmt.Sprintf("[%5v] init gateway config success", chainID), "isReload", isReload)
}

// InitChainConfig impl
func InitChainConfig(b tokens.IBridge, chainID *big.Int) {
	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(isReload, log.Error, log.Fatal)
	chainCfg, err := router.GetChainConfig(chainID)
	if err != nil {
		logErrFunc("get chain config failed", "chainID", chainID, "err", err)
		if isReload {
			return
		}
	}
	if chainCfg == nil {
		logErrFunc("chain config not found", "chainID", chainID)
		if isReload {
			return
		}
	}
	if chainID.String() != chainCfg.ChainID {
		logErrFunc("verify chain ID mismatch", "inconfig", chainCfg.ChainID, "inchainids", chainID)
		if isReload {
			return
		}
	}
	if err = chainCfg.CheckConfig(); err != nil {
		logErrFunc("check chain config failed", "chainID", chainID, "err", err)
		if isReload {
			return
		}
	}
	b.SetChainConfig(chainCfg)
	log.Info("init chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID, "isReload", isReload)

	routerContract := chainCfg.RouterContract
	if !isRouterInfoLoaded(routerContract) {
		err = b.InitRouterInfo(routerContract)
		if err == nil {
			routerInfoIsLoaded.Store(routerContract, struct{}{})
		} else {
			logErrFunc("init chain router info failed", "routerContract", routerContract, "err", err)
			if isReload {
				return
			}
		}
	}
}

// InitTokenConfig impl
//nolint:funlen,gocyclo // allow long init token config method
func InitTokenConfig(b tokens.IBridge, tokenID string, chainID *big.Int) {
	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(isReload, log.Error, log.Fatal)
	if tokenID == "" {
		logErrFunc("empty token ID")
		if isReload {
			return
		}
	}
	tokenAddr, err := router.GetMultichainToken(tokenID, chainID)
	if err != nil {
		logErrFunc("get token address failed", "tokenID", tokenID, "chainID", chainID, "err", err)
		if isReload {
			return
		}
	}
	if common.HexToAddress(tokenAddr) == (common.Address{}) {
		log.Debugf("[%5v] '%v' token address is empty", chainID, tokenID)
		return
	}
	tokenCfg, err := router.GetTokenConfig(chainID, tokenID)
	if err != nil {
		logErrFunc("get token config failed", "chainID", chainID, "tokenID", tokenID, "err", err)
		if isReload {
			return
		}
	}
	if tokenCfg == nil {
		log.Debug("token config not found", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr)
		return
	}
	if common.HexToAddress(tokenAddr) != common.HexToAddress(tokenCfg.ContractAddress) {
		logErrFunc("verify token address mismach", "tokenID", tokenID, "chainID", chainID, "inconfig", tokenCfg.ContractAddress, "inmultichain", tokenAddr)
		if isReload {
			return
		}
	}
	if tokenID != tokenCfg.TokenID {
		logErrFunc("verify token ID mismatch", "chainID", chainID, "inconfig", tokenCfg.TokenID, "intokenids", tokenID)
		if isReload {
			return
		}
	}
	if err = tokenCfg.CheckConfig(); err != nil {
		logErrFunc("check token config failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
		if isReload {
			return
		}
	}
	routerContract, err := router.GetCustomConfig(chainID, tokenAddr)
	if err != nil {
		logErrFunc("get custom config failed", "chainID", chainID, "key", tokenAddr, "err", err)
		if isReload {
			return
		}
	}

	tokenCfg.RouterContract = routerContract
	b.SetTokenConfig(tokenAddr, tokenCfg)

	router.SetMultichainToken(tokenID, chainID.String(), tokenAddr)

	log.Info(fmt.Sprintf("[%5v] init '%v' token config success", chainID, tokenID), "tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals)

	if !isRouterInfoLoaded(routerContract) {
		err = b.InitRouterInfo(routerContract)
		if err == nil {
			routerInfoIsLoaded.Store(routerContract, struct{}{})
		} else {
			logErrFunc("init token router info failed", "routerContract", routerContract, "err", err)
			if isReload {
				return
			}
		}
	}
}

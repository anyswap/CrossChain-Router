// Package bridge init router bridge and load / reload configs.
package bridge

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

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

func getRouterInfoLoadedKey(chainID, routerContract string) string {
	return strings.ToLower(fmt.Sprintf("%s:%s", chainID, routerContract))
}

func isRouterInfoLoaded(chainID, routerContract string) bool {
	key := getRouterInfoLoadedKey(chainID, routerContract)
	_, exist := routerInfoIsLoaded.Load(key)
	return exist
}

func setRouterInfoLoaded(chainID, routerContract string) {
	key := getRouterInfoLoadedKey(chainID, routerContract)
	routerInfoIsLoaded.Store(key, struct{}{})
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

	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)

	client.InitHTTPClient()
	router.InitRouterConfigClients()

	allChainIDs, err := router.GetAllChainIDs()
	if err != nil {
		logErrFunc("call GetAllChainIDs failed", "err", err)
		return
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
		return
	}

	allTokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		logErrFunc("call GetAllTokenIDs failed", "err", err)
		return
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
		return
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

	loadSwapAndFeeConfigs()

	mpc.Init(isServer)

	success = true
}

func loadSwapAndFeeConfigs() {
	if !tokens.IsERC20Router() {
		return
	}

	swapConfigs := new(sync.Map)
	feeConfigs := new(sync.Map)

	wg := new(sync.WaitGroup)
	for _, tokenID := range router.AllTokenIDs {
		supportChainIDs := make([]*big.Int, 0, len(router.AllChainIDs))
		for _, chainID := range router.AllChainIDs {
			multichainToken := router.GetCachedMultichainToken(tokenID, chainID.String())
			if multichainToken != "" {
				supportChainIDs = append(supportChainIDs, chainID)
			}
		}
		if len(supportChainIDs) == 0 {
			continue
		}

		tokenIDSwapConfig := new(sync.Map)
		swapConfigs.Store(tokenID, tokenIDSwapConfig)

		tokenIDFeeConfig := new(sync.Map)
		feeConfigs.Store(tokenID, tokenIDFeeConfig)

		wg.Add(2)
		go loadSwapConfigs(wg, tokenIDSwapConfig, tokenID, supportChainIDs)
		go loadFeeConfigs(wg, tokenIDFeeConfig, tokenID, supportChainIDs)
	}
	wg.Wait()

	tokens.SetSwapConfigs(swapConfigs)
	tokens.SetFeeConfigs(feeConfigs)

	log.Info("load all swap and fee config success")
}

//nolint:dupl // allow duplicate
func loadSwapConfigs(wg *sync.WaitGroup, swapConfigs *sync.Map, tokenID string, supportChainIDs []*big.Int) {
	defer wg.Done()

	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)

	wg2 := new(sync.WaitGroup)
	for i, fromChainID := range supportChainIDs {
		fmap := new(sync.Map)
		swapConfigs.Store(fromChainID.String(), fmap)
		for j, toChainID := range supportChainIDs {
			if i == j {
				continue
			}
			wg2.Add(1)
			go func(wg *sync.WaitGroup, tokenID string, fromChainID, toChainID *big.Int) {
				defer wg.Done()
				swapCfg, err := router.GetActualSwapConfig(tokenID, fromChainID, toChainID)
				if err != nil {
					logErrFunc("get swap config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return
				}
				err = swapCfg.CheckConfig()
				if err != nil {
					logErrFunc("check swap config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return
				}
				fmap.Store(toChainID.String(), swapCfg)
				log.Info("load swap config success", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID)
			}(wg2, tokenID, fromChainID, toChainID)
		}
	}
	wg2.Wait()
}

//nolint:dupl // allow duplicate
func loadFeeConfigs(wg *sync.WaitGroup, feeConfigs *sync.Map, tokenID string, supportChainIDs []*big.Int) {
	defer wg.Done()

	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)

	wg2 := new(sync.WaitGroup)
	for i, fromChainID := range supportChainIDs {
		fmap := new(sync.Map)
		feeConfigs.Store(fromChainID.String(), fmap)
		for j, toChainID := range supportChainIDs {
			if i == j {
				continue
			}
			wg2.Add(1)
			go func(wg *sync.WaitGroup, tokenID string, fromChainID, toChainID *big.Int) {
				defer wg.Done()
				feeCfg, err := router.GetActualFeeConfig(tokenID, fromChainID, toChainID)
				if err != nil {
					logErrFunc("get fee config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return
				}
				err = feeCfg.CheckConfig()
				if err != nil {
					logErrFunc("check fee config failed", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID, "err", err)
					return
				}
				fmap.Store(toChainID.String(), feeCfg)
				log.Info("load fee config success", "tokenID", tokenID, "fromChainID", fromChainID, "toChainID", toChainID)
			}(wg2, tokenID, fromChainID, toChainID)
		}
	}
	wg2.Wait()
}

// InitGatewayConfig impl
func InitGatewayConfig(b tokens.IBridge, chainID *big.Int) {
	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)
	if chainID == nil || chainID.Sign() == 0 {
		logErrFunc("init gateway with zero chain ID")
		return
	}
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		logErrFunc("gateway not found for chain ID", "chainID", chainID)
		return
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
			return
		}
		log.Infof("[%5v] lastest block number is %v", chainID, latestBlock)
	}
	log.Info(fmt.Sprintf("[%5v] init gateway config success", chainID), "isReload", isReload)
}

// InitChainConfig impl
func InitChainConfig(b tokens.IBridge, chainID *big.Int) {
	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)
	chainCfg, err := router.GetChainConfig(chainID)
	if err != nil {
		logErrFunc("get chain config failed", "chainID", chainID, "err", err)
		return
	}
	if chainCfg == nil {
		logErrFunc("chain config not found", "chainID", chainID)
		return
	}
	if chainID.String() != chainCfg.ChainID {
		logErrFunc("verify chain ID mismatch", "inconfig", chainCfg.ChainID, "inchainids", chainID)
		return
	}
	if err = chainCfg.CheckConfig(); err != nil {
		logErrFunc("check chain config failed", "chainID", chainID, "err", err)
		return
	}
	b.SetChainConfig(chainCfg)
	log.Info("init chain config success", "blockChain", chainCfg.BlockChain, "chainID", chainID, "isReload", isReload)

	routerContract := chainCfg.RouterContract
	if routerContract != "" && !isRouterInfoLoaded(chainID.String(), routerContract) {
		err = b.InitRouterInfo(routerContract)
		if err == nil {
			setRouterInfoLoaded(chainID.String(), routerContract)
		} else {
			logErrFunc("init chain router info failed", "routerContract", routerContract, "err", err)
			return
		}
	}
}

// InitTokenConfig impl
//nolint:funlen,gocyclo // allow long init token config method
func InitTokenConfig(b tokens.IBridge, tokenID string, chainID *big.Int) {
	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(router.DontPanicInLoading(), log.Error, log.Fatal)
	if tokenID == "" {
		logErrFunc("empty token ID")
		return
	}
	tokenAddr, err := router.GetMultichainToken(tokenID, chainID)
	if err != nil {
		logErrFunc("get token address failed", "tokenID", tokenID, "chainID", chainID, "err", err)
		return
	}
	if tokenAddr == "" {
		log.Debugf("[%5v] '%v' token address is empty", chainID, tokenID)
		return
	}
	tokenCfg, err := router.GetTokenConfig(chainID, tokenID)
	if err != nil {
		logErrFunc("get token config failed", "chainID", chainID, "tokenID", tokenID, "err", err)
		return
	}
	if tokenCfg == nil {
		log.Debug("token config not found", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr)
		return
	}
	if !strings.EqualFold(tokenAddr, tokenCfg.ContractAddress) {
		logErrFunc("verify token address mismach", "tokenID", tokenID, "chainID", chainID, "inconfig", tokenCfg.ContractAddress, "inmultichain", tokenAddr)
		return
	}
	if tokenID != tokenCfg.TokenID {
		logErrFunc("verify token ID mismatch", "chainID", chainID, "inconfig", tokenCfg.TokenID, "intokenids", tokenID)
		return
	}
	if err = tokenCfg.CheckConfig(); err != nil {
		logErrFunc("check token config failed", "tokenID", tokenID, "chainID", chainID, "tokenAddr", tokenAddr, "err", err)
		return
	}
	b.SetTokenConfig(tokenAddr, tokenCfg)

	router.SetMultichainToken(tokenID, chainID.String(), tokenAddr)

	log.Info(fmt.Sprintf("[%5v] init '%v' token config success", chainID, tokenID), "tokenAddr", tokenAddr, "decimals", tokenCfg.Decimals, "isReload", isReload)

	routerContract := tokenCfg.RouterContract
	if routerContract != "" && !isRouterInfoLoaded(chainID.String(), routerContract) {
		err = b.InitRouterInfo(routerContract)
		if err == nil {
			setRouterInfoLoaded(chainID.String(), routerContract)
		} else {
			logErrFunc("init token router info failed", "routerContract", routerContract, "err", err)
			return
		}
	}
}

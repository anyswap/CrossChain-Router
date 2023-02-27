package bridge

import (
	"fmt"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools"
)

var (
	adjustInterval = 60 // seconds

	adjustGatewayChains = new(sync.Map)
)

// AdjustGatewayOrder adjust gateway order once
func AdjustGatewayOrder(bridge tokens.IBridge, chainID string) {
	if IsWrapperMode {
		return
	}
	// use block number as weight
	var weightedAPIs tools.WeightedStringSlice
	gateway := bridge.GetGatewayConfig()
	if gateway == nil {
		return
	}
	var maxHeight uint64
	length := len(gateway.APIAddress)
	for i := length; i > 0; i-- { // query in reverse order
		if utils.IsCleanuping() {
			return
		}
		apiAddress := gateway.APIAddress[i-1]
		height, _ := bridge.GetLatestBlockNumberOf(apiAddress)
		weightedAPIs = weightedAPIs.Add(apiAddress, height)
		if height > maxHeight {
			maxHeight = height
		}
	}
	if length == 0 { // update for bridges only use grpc apis
		maxHeight, _ = bridge.GetLatestBlockNumber()
	}
	if maxHeight > 0 {
		log.Info("update latest block number", "chainID", chainID, "height", maxHeight)
		router.CachedLatestBlockNumber.Store(chainID, maxHeight)
	}
	if len(weightedAPIs) > 0 {
		weightedAPIs.Reverse() // reverse as iter in reverse order in the above
		weightedAPIs = weightedAPIs.Sort()
		gateway.APIAddress = weightedAPIs.GetStrings()
		gateway.WeightedAPIs = weightedAPIs
	}

	if _, exist := adjustGatewayChains.Load(chainID); !exist {
		adjustGatewayChains.Store(chainID, struct{}{})
		go adjustGatewayOrder(bridge, chainID)
	}
}

func adjustGatewayOrder(bridge tokens.IBridge, chainID string) {
	for adjustCount := 0; ; adjustCount++ {
		for i := 0; i < adjustInterval; i++ {
			if utils.IsCleanuping() {
				return
			}
			time.Sleep(1 * time.Second)
		}

		AdjustGatewayOrder(bridge, chainID)

		if adjustCount%3 == 0 && !params.IsWrapperGateway(chainID) {
			log.Info(fmt.Sprintf("adjust gateways of chain %v", chainID), "result", bridge.GetGatewayConfig().WeightedAPIs)
		}
	}
}

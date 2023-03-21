package bridge

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools"
)

var (
	adjustInterval = 60 // seconds

	adjustGatewayChains = new(sync.Map)
)

type gatewayQuality struct {
	ContinueFailues uint64 `json:",omitempty"`
}

type gatewayQualityMap map[string]*gatewayQuality

func (m gatewayQualityMap) MarshalJSON() ([]byte, error) {
	t := make(map[string]*gatewayQuality)
	for k, v := range m {
		if v.ContinueFailues == 0 {
			continue
		}
		t[k] = v
	}
	return json.Marshal(t)
}

func (m gatewayQualityMap) String() string {
	data, _ := json.Marshal(m)
	return strings.ReplaceAll(string(data), `"`, "")
}

type adjustContext struct {
	WeightedAPIs   tools.WeightedStringSlice
	GatewayQuality gatewayQualityMap
}

// AdjustGatewayOrder adjust gateway order once
func AdjustGatewayOrder(bridge tokens.IBridge, chainID string) {
	// use block number as weight
	var weightedAPIs tools.WeightedStringSlice
	gateway := bridge.GetGatewayConfig()
	if gateway == nil {
		return
	}
	var adjustCtx *adjustContext
	if gateway.AdjustContext == nil {
		gateway.AdjustContext = &adjustContext{
			GatewayQuality: make(map[string]*gatewayQuality),
		}
	}
	adjustCtx = gateway.AdjustContext.(*adjustContext)
	var maxHeight uint64
	originURLs := gateway.OriginAllGatewayURLs
	for i := len(originURLs); i > 0; i-- { // query in reverse order
		if utils.IsCleanuping() {
			return
		}
		apiAddress := originURLs[i-1]
		if adjustCtx.GatewayQuality[apiAddress] == nil {
			adjustCtx.GatewayQuality[apiAddress] = &gatewayQuality{}
		}

		height, err := bridge.GetLatestBlockNumberOf(apiAddress)
		if err != nil {
			adjustCtx.GatewayQuality[apiAddress].ContinueFailues++
			if adjustCtx.GatewayQuality[apiAddress].ContinueFailues >= 3 {
				if adjustCtx.GatewayQuality[apiAddress].ContinueFailues == 3 {
					log.Warn("remove low quality gateway", "url", apiAddress, "chainID", chainID)
				}
				continue
			}
		} else {
			if adjustCtx.GatewayQuality[apiAddress].ContinueFailues > 0 {
				log.Info("recover low quality gateway", "url", apiAddress, "chainID", chainID)
				adjustCtx.GatewayQuality[apiAddress].ContinueFailues = 0
			}
		}

		weightedAPIs = weightedAPIs.Add(apiAddress, height)
		if height > maxHeight {
			maxHeight = height
		}
	}
	if len(originURLs) == 0 { // update for bridges only use grpc apis
		maxHeight, _ = bridge.GetLatestBlockNumber()
	}
	if maxHeight > 0 {
		router.CachedLatestBlockNumber.Store(chainID, maxHeight)
	}
	if weightedAPIs.Len() > 0 {
		weightedAPIs.Reverse() // reverse as iter in reverse order in the above
		weightedAPIs = weightedAPIs.Sort()
		gateway.AllGatewayURLs = weightedAPIs.GetStrings()
	} else if len(originURLs) > 0 {
		// no one is usable, then recover to the original state
		gateway.AllGatewayURLs = gateway.OriginAllGatewayURLs
		log.Info("reset to original gateways", "chainID", chainID, "count", len(gateway.AllGatewayURLs))
	}
	adjustCtx.WeightedAPIs = weightedAPIs

	if _, exist := adjustGatewayChains.Load(chainID); !exist {
		log.Info(fmt.Sprintf("adjust gateways of chain %v", chainID), "result", adjustCtx.WeightedAPIs, "gatewayQuality", adjustCtx.GatewayQuality)

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

		if adjustCount%3 == 0 {
			adjustCtx := bridge.GetGatewayConfig().AdjustContext.(*adjustContext)
			log.Info(fmt.Sprintf("adjust gateways of chain %v", chainID), "result", adjustCtx.WeightedAPIs, "gatewayQuality", adjustCtx.GatewayQuality)
		}
	}
}

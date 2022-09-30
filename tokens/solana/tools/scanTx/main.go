package main

import (
	"flag"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
)

var (
	bridge = solana.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string

	routerProgramID string

	mpcConfig *mpc.Config
	chainID   = big.NewInt(0)
)

func main() {
	initAll()

	var start uint64
	var end uint64
	for i := 0; i < 5; i++ {

		ei, err := bridge.GetEpochInfo()
		if err != nil {
			log.Error("GetEpochInfo error %v", err)
		} else {
			fmt.Println("EpochInfo", ei)
			if start == 0 {
				start = uint64(ei.AbsoluteSlot)
			}
			end = uint64(ei.AbsoluteSlot)
		}

		if start < end {
			blocks, err := bridge.GetBlocks(start+1, end)
			if err != nil {
				log.Errorf("GetBlocks error %v", err)
			}
			fmt.Println("GetBlocks ", start, "--", end, blocks)
			start = uint64(ei.AbsoluteSlot)

			for _, block := range *blocks {
				result, err := bridge.GetBlock(block, false)
				if err != nil {
					log.Errorf("GetBlock error %v", err)
				}

				for _, v := range result.Transactions {
					if v.Transaction.Message.AccountKeys[len(v.Transaction.Message.AccountKeys)-1].String() == routerProgramID {
						fmt.Println(v.Transaction.Signatures)
					}
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&routerProgramID, "router", "", "router program id")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

	log.Info("init flags finished")
}

func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	if config.FastMPC != nil {
		mpcConfig = mpc.InitConfig(config.FastMPC, true)
	} else {
		mpcConfig = mpc.InitConfig(config.MPC, true)
	}
	log.Info("init config finished, IsFastMPC: %v", mpcConfig.IsFastMPC)
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

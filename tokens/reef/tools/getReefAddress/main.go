package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/reef"
)

var (
	bridge = reef.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string

	paramPublicKey string

	mpcConfig *mpc.Config
)

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&paramPublicKey, "pubkey", "", "mpc public key")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)
	initAll()

	bridge.InitAfterConfig()

	fmt.Printf("pubkey: %s \n", paramPublicKey)

	reefAddr := reef.PubkeyToReefAddress(paramPublicKey)
	fmt.Printf("reefAddr: %s \n", reefAddr)

	evmAddr, err := bridge.PublicKeyToAddress(paramPublicKey)
	if err != nil {
		panic(err)
	}
	fmt.Printf("calc evmAddr: %s \n", evmAddr)

	bindAddr, err := bridge.QueryEvmAddress(reefAddr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("bind evmAddr: %s \n", bindAddr.LowerHex())

}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}
func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	if config.FastMPC != nil {
		mpcConfig = mpc.InitConfig(config.FastMPC, true)
	} else {
		mpcConfig = mpc.InitConfig(config.MPC, true)
	}
	log.Info("init config finished", "IsFastMPC", mpcConfig.IsFastMPC)
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[paramChainID]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", paramChainID)
	}
	apiAddrsExt := cfg.GatewaysExt[paramChainID]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	bridge.SetChainConfig(&tokens.ChainConfig{
		ChainID: paramChainID,
	})
	log.Info("init bridge finished")
}

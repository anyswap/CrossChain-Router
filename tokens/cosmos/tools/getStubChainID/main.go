package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos"
)

var (
	paramChainName string
	paramNetwork   string
)

func initFlags() {
	flag.StringVar(&paramChainName, "n", "", "chainName, eg. cosmoshub, osmosis, coreum, sei, etc.")
	flag.StringVar(&paramNetwork, "p", "", "network, eg. mainnet, testnet, etc.")

	flag.Parse()
}

func main() {
	initFlags()

	if !cosmos.IsSupportedCosmosSubChain(paramChainName) {
		log.Fatalf("unknown chain name %v", paramChainName)
	}

	network := paramNetwork
	if network == "" && len(os.Args) > 1 {
		network = os.Args[1]
	}
	if network == "" {
		log.Fatal("miss network argument")
	}

	chainID := cosmos.GetStubChainID(paramChainName, network)
	fmt.Printf("%v %v: %v\n", paramChainName, network, chainID)
}

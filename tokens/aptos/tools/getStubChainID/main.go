package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
)

var (
	paramNetwork string
)

func initFlags() {
	flag.StringVar(&paramNetwork, "p", "", "network, eg. mainnet, testnet, devnet, etc.")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	network := paramNetwork
	if network == "" && len(os.Args) > 1 {
		network = os.Args[1]
	}
	if network == "" {
		for _, v := range []string{"mainnet", "testnet", "devnet"} {
			chainID := aptos.GetStubChainID(v)
			fmt.Printf("%v: %v\n", v, chainID)
		}
	} else {
		chainID := aptos.GetStubChainID(network)
		fmt.Printf("%v: %v\n", network, chainID)
	}
}

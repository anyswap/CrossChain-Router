package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tron"
)

var (
	paramEthAddress string
)

func initFlags() {
	flag.StringVar(&paramEthAddress, "ethAddr", "", "eth address")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	ethAddress := paramEthAddress
	if ethAddress == "" && len(os.Args) > 1 {
		ethAddress = os.Args[1]
	}
	if ethAddress == "" {
		log.Fatal("miss eth address argument")
	}

	tronAddr, err := tron.EthToTron(ethAddress)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("eth address: %v\n", tronAddr)
}

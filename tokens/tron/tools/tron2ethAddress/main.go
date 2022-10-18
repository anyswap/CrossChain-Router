package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tron"
)

var (
	paramTronAddress string
)

func initFlags() {
	flag.StringVar(&paramTronAddress, "tronAddr", "", "tron address (base58 encoded)")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	tronAddress := paramTronAddress
	if tronAddress == "" && len(os.Args) > 1 {
		tronAddress = os.Args[1]
	}
	if tronAddress == "" {
		log.Fatal("miss tron address argument")
	}

	ethAddr, err := tron.TronToEth(tronAddress)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("eth address: %v\n", ethAddr)
}

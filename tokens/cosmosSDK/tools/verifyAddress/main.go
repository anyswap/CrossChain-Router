package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
)

var (
	paramPrefix  string
	paramAddress string
)

func initFlags() {
	flag.StringVar(&paramPrefix, "prefix", "", "prefix, eg. cosmos, sei, etc.")
	flag.StringVar(&paramAddress, "address", "", "address")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	res := cosmosSDK.IsValidAddress(paramPrefix, paramAddress)
	fmt.Printf("address:%s is valid address:%v\n", paramAddress, res)
}

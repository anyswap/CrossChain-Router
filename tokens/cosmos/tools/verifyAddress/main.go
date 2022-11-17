package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos"
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
	initFlags()

	res := cosmos.IsValidAddress(paramPrefix, paramAddress)
	fmt.Printf("address:%s is valid address:%v\n", paramAddress, res)
}

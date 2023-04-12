package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/iota"
)

var (
	paramPubKey string
	paramPrefix string
)

func initFlags() {
	flag.StringVar(&paramPubKey, "publicKey", "", "public key string")
	flag.StringVar(&paramPrefix, "prefix", "", "address prefix, eg. atoi")
	flag.Parse()
}

func main() {
	initFlags()

	addr, err := iota.HexPublicKeyToAddress(paramPrefix, paramPubKey)
	if err != nil {
		log.Fatalf("convert to address err: %v\n", err)
	}
	fmt.Printf("addr: %v\n", addr)
}

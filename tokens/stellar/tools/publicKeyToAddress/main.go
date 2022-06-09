package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/stellar"
)

var (
	paramPubKey string
)

func initFlags() {
	flag.StringVar(&paramPubKey, "p", "", "public key string")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	addr, err := stellar.PublicKeyHexToAddress(paramPubKey)
	if err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("address: %v\n", addr)
}

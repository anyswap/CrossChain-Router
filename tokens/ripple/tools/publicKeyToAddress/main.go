package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple"
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

	pubkey := paramPubKey
	if pubkey == "" && len(os.Args) > 1 {
		pubkey = os.Args[1]
	}
	if pubkey == "" {
		log.Fatal("miss public key argument")
	}

	addr, err := ripple.PublicKeyHexToAddress(pubkey)
	if err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("address: %v\n", addr)
}

package main

import (
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple"
)

func main() {
	log.SetLogger(6, false, true)
	if len(os.Args) < 2 {
		log.Fatal("must provide a public key hex string argument")
	}

	pubkeyHex := os.Args[1]

	addr, err := ripple.PublicKeyHexToAddress(pubkeyHex)
	if err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("address: %v\n", addr)
}

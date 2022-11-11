package main

import (
	"flag"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/near"
)

var (
	paramPubKey string
)

func initFlags() {
	flag.StringVar(&paramPubKey, "p", "", "public key hex string")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	pubkeyHex := paramPubKey
	if pubkeyHex == "" && len(os.Args) > 1 {
		pubkeyHex = os.Args[1]
	}
	if pubkeyHex == "" {
		log.Fatal("miss public key argument")
	}

	nearPubKey, err := near.PublicKeyFromHexString(pubkeyHex)
	if err != nil {
		log.Fatal("convert public key to address failed", "err", err)
	}

	log.Info("convert public key to address success")
	log.Printf("nearAddress is %v", nearPubKey.Address())
	log.Printf("nearPublicKey is %v", nearPubKey.String())
}

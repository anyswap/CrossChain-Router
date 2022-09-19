package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
)

var (
	paramPublicKey string
)

func initFlags() {
	flag.StringVar(&paramPublicKey, "p", "", "publicKey")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	if addr, err := cosmosSDK.PublicKeyToAddress(paramPublicKey); err != nil {
		log.Fatal("err:%v \n", err)
	} else {
		fmt.Printf("addr:%v \n", addr)
	}
}

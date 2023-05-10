package main

import (
	"encoding/hex"
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/stellar/go/strkey"
)

var (
	paramAddress string
)

func initFlags() {
	flag.StringVar(&paramAddress, "a", "", "address string")
	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()
	fmt.Printf("address: %v\n", paramAddress)
	publicKey, err := strkey.Decode(strkey.VersionByteAccountID, paramAddress)
	if err != nil {
		log.Fatalf("%v", err)
	}
	pub := hex.EncodeToString(publicKey)

	fmt.Printf("publickey: %v\n", pub)
}

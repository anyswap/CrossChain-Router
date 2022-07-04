package main

import (
	"encoding/hex"
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/block"
)

var (
	paramWif     string
	paramChainID string
)

func initFlags() {
	flag.StringVar(&paramWif, "wif", "", "wif")
	flag.StringVar(&paramChainID, "chainID", "", "chainID")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	if paramWif == "" {
		log.Fatal("miss network argument")
	}

	wifPd, err := block.DecodeWIF(paramWif)
	if err != nil {
		log.Fatal("DecodeWIF fails", "paramWif", paramWif)
	}
	ecPrikey := wifPd.PrivKey.ToECDSA()
	priString := hex.EncodeToString(ecPrikey.D.Bytes())
	fmt.Printf("%v: %v:\n", paramWif, priString)
}

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/stellar/go/txnbuild"
)

var (
	paramAddress string
	paramChainID string
)

func initFlags() {
	flag.StringVar(&paramAddress, "a", "", "address string")
	flag.StringVar(&paramChainID, "c", "", "chain id string")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	rtn := new(txnbuild.MemoHash)
	chainId, ok := new(big.Int).SetString(paramChainID, 10)
	if !ok {
		log.Fatal("paramChainID format error")
	}
	if paramAddress[:2] == "0x" || paramAddress[:2] == "0X" {
		paramAddress = paramAddress[2:]
	}
	b, err := hex.DecodeString(paramAddress)
	if err != nil {
		log.Fatal("paramAddress is not hex string")
	}
	c := chainId.Bytes()
	if len(b)+len(c) > 31 {
		log.Fatal("build memo error")
	}

	rtn[0] = byte(len(b))
	for i := 0; i < len(b); i++ {
		rtn[i+1] = b[i]
	}
	for i := 0; i < len(c); i++ {
		rtn[32-len(c)+i] = c[i]
	}
	memo := hex.EncodeToString(rtn[:])
	fmt.Printf("memo: %v\n", memo)
}

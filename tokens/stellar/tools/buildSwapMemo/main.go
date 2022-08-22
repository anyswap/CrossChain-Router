package main

import (
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/stellar/go/txnbuild"
)

var (
	paramAddress string
	paramChainID string
	memobase64   string
)

func initFlags() {
	flag.StringVar(&paramAddress, "a", "", "address string")
	flag.StringVar(&paramChainID, "c", "", "chain id string")
	flag.StringVar(&memobase64, "m", "", "memo base64 string")

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
	fmt.Printf("chainId: %v  bytes: %v  \n", chainId, chainId.Bytes())
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
	fmt.Printf("memobytes: %v\n", rtn)
	fmt.Printf("memo: %v\n", memo)

	var memobytes []byte
	if len(memobase64) == 0 {
		fmt.Printf("memo: %v \n", memo)
		memobytes = rtn[:]
	} else {
		fmt.Printf("memobase64: %v \n", memobase64)
		memobytes, err = base64.StdEncoding.DecodeString(memobase64)
		if err != nil || len(memobytes) == 0 {
			log.Fatal("parse memo error")
		}
	}
	fmt.Printf("memobytes: %v \n", memobytes)

	addrLen := int(memobytes[0])
	addEnd := 1 + addrLen
	if len(memobytes) < addEnd+1 {
		log.Fatal("parse memo length error")
	}
	bindStr := common.ToHex(memobytes[1:addEnd])
	bigToChainID := new(big.Int).SetBytes(memobytes[addEnd:])
	fmt.Printf("bindStr: %v , toChainID: %v \n", bindStr, bigToChainID)
}

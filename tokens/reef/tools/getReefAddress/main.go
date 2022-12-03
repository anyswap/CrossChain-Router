package main

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens/reef"
)

func main() {
	addr := reef.PubkeyToReefAddress("0x62c48aa955218081a6e168b8808d641fd9994ea226d9d572383d03bd1fdad747")
	fmt.Println(addr)
	fmt.Println(common.Bytes2Hex(reef.AddressToPubkey(addr)))
}

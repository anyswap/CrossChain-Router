package main

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

func main() {
	log.SetLogger(6, false, true)
	mintPublicKey, mintPriKey, _ := types.NewRandomPrivateKey()
	fmt.Printf("PriKey bytes: [")
	for i := 0; i < len(mintPriKey); i++ {
		if i == len(mintPriKey)-1 {
			fmt.Printf("%v", mintPriKey[i])
		} else {
			fmt.Printf("%v, ", mintPriKey[i])
		}
	}
	fmt.Printf("] \n")
	fmt.Printf("PriKey(base58): %v\n", mintPriKey.String())
	fmt.Printf("Address(base58): %v\n", mintPublicKey.String())
}

package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	paramPubKey  string
	paramNetwork string
)

func initFlags() {
	flag.StringVar(&paramPubKey, "publicKey", "", "public key string")
	flag.StringVar(&paramNetwork, "p", "", "network url")
	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	// create a new node API client
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(paramNetwork)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()

	// fetch the node's info to know the min. required PoW score
	if info, err := nodeHTTPAPIClient.Info(ctx); err != nil {
		log.Fatal("Info", "paramNetwork", paramNetwork, "err", err)
	} else {
		fmt.Printf("info: %+v\n", info)

		if publicKey, err := hex.DecodeString(paramPubKey); err != nil {
			log.Fatal("DecodeString", "paramPubKey", paramPubKey, "err", err)
		} else {
			edAddr := iotago.AddressFromEd25519PubKey(publicKey)
			bech32Addr := edAddr.Bech32(iotago.NetworkPrefix(info.Bech32HRP))
			fmt.Printf("edAddr: %+v\niotaAddr: %+v\n", edAddr.String(), bech32Addr)
		}
	}
}

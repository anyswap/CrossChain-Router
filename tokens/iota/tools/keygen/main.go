package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/iota"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

var (
	paramNetwork string
	addrPrefix   string
)

func initFlags() {
	flag.StringVar(&paramNetwork, "p", "devnet", "network, eg. mainnet, or devnet.")
	flag.Parse()
}

func main() {
	initFlags()

	network := paramNetwork
	switch network {
	case "mainnet":
		addrPrefix = string(iotago.PrefixMainnet)
	case "devnet":
		addrPrefix = string(iotago.PrefixTestnet)
	default:
		log.Fatal("invalid network, choose mainnet or devnet")
	}

	pubKey, privKey, err := GenerateKey()
	if err != nil {
		log.Fatalf("key gen failed, error: %v", err)
	}

	pubKeyStr := common.Bytes2Hex(pubKey)
	addr := iota.ConvertPubKeyToAddr(pubKeyStr)
	bech32Addr := addr.Bech32(iotago.NetworkPrefix(addrPrefix))
	fmt.Printf("Generated random keys: \n")
	fmt.Printf("Address: %s\n", bech32Addr)
	fmt.Printf("Public Key: %s\n", pubKeyStr)
	fmt.Printf("Private Key: %s\n", common.Bytes2Hex(privKey))
}

func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	rand.Seed(time.Now().UnixNano())
	seed := make([]byte, ed25519.SeedSize)
	rand.Read(seed)
	return ed25519.GenerateKey(bytes.NewReader(seed))
}

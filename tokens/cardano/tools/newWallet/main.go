package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/cosmos/go-bip39"
	"github.com/echovl/cardano-go"
	"github.com/echovl/cardano-go/crypto"
)

var (
	paramNetwork string
)

const (
	entropySizeInBits         = 160
	purposeIndex       uint32 = 1852 + 0x80000000
	coinTypeIndex      uint32 = 1815 + 0x80000000
	accountIndex       uint32 = 0x80000000
	externalChainIndex uint32 = 0x0
)

func initFlags() {
	flag.StringVar(&paramNetwork, "p", "", "network, eg. mainnet, testnet, etc.")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	network := cardano.Mainnet
	if paramNetwork != "mainnet" {
		network = cardano.Testnet
	}

	entropy, _ := bip39.NewEntropy(entropySizeInBits)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	fmt.Println("mnemonic", mnemonic)
	rootKey := crypto.NewXPrvKeyFromEntropy(entropy, "")
	accountKey := rootKey.Derive(purposeIndex).
		Derive(coinTypeIndex).
		Derive(accountIndex)
	chainKey := accountKey.Derive(externalChainIndex)
	stakeKey := accountKey.Derive(2).Derive(0)
	addr0Key := chainKey.Derive(0)

	payment, err := cardano.NewKeyCredential(addr0Key.PubKey())
	if err != nil {
		panic(err)
	}
	enterpriseAddr, err := cardano.NewEnterpriseAddress(network, payment)
	if err != nil {
		panic(err)
	}
	fmt.Println("addr", enterpriseAddr.String())

	fmt.Println("private key", addr0Key.PrvKey().Bech32("addr_sk"))
	fmt.Println("public key", stakeKey.PrvKey().Bech32("addr_vk"))
}

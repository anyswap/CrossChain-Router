package main

import (
	"flag"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	cardanosdk "github.com/echovl/cardano-go"
	"github.com/echovl/cardano-go/crypto"
)

var (
	paramAsset     string
	paramPolicyKey string
	appendName     bool
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	if appendName {
		paramPolicyKey = paramPolicyKey + paramAsset
	}
	policyKey := crypto.NewXPrvKeyFromEntropy([]byte(paramPolicyKey), "")
	policyScript, err := cardanosdk.NewScriptPubKey(policyKey.PubKey())
	if err != nil {
		panic(err)
	}
	policyID, err := cardanosdk.NewPolicyID(policyScript)
	if err != nil {
		panic(err)
	}
	assetName := cardanosdk.NewAssetName(paramAsset)

	assetNameWithPolicy := policyID.String() + "." + common.Bytes2Hex(assetName.Bytes())
	fmt.Printf("asset: %s", assetNameWithPolicy)

}

func initAll() {
	initFlags()
}

func initFlags() {
	flag.BoolVar(&appendName, "append", false, "append asset name")
	flag.StringVar(&paramAsset, "asset", "", "asset eg. USDT")
	flag.StringVar(&paramPolicyKey, "key", "", "policy seed")

	flag.Parse()

	log.Info("init flags finished")
}

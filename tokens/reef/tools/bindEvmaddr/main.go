package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/reef"
)

var (
	bridge = reef.NewCrossChainBridge()

	url string

	paramConfigFile string
	paramChainID    string

	paramPublicKey     string
	paramEvmPrivateKey string
	jsPath             string

	mpcConfig *mpc.Config
)

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")

	flag.StringVar(&paramEvmPrivateKey, "evmPrivateKey", "", "private key of evm address")
	flag.StringVar(&jsPath, "jspath", "", "js path")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)
	initAll()

	reef.InstallJSModules(jsPath, url)
	out, err := reef.BindEvmAddr(paramPublicKey, paramEvmPrivateKey)
	if err != nil {
		panic(err.Error())
	}
	var signInfo = out[0]
	var signingMessage = out[1]
	fmt.Println(signInfo, signingMessage)

	jsondata, _ := json.Marshal(tokens.BuildTxArgs{})
	msgContext := string(jsondata)
	keyID, rsvs, err := mpcConfig.DoSignOne(reef.MPC_PUBLICKEY_TYPE, paramPublicKey, signingMessage, msgContext)
	if err != nil {
		panic(err)
	}
	if len(rsvs) != 1 {
		panic("get sign status require one rsv but return many")
	}

	rsv := rsvs[0]
	fmt.Println(rsv, keyID)

	params := strings.Split(signInfo, " ")

	txhash, err := reef.SendBindEvm(paramPublicKey, paramEvmPrivateKey, params[0], params[1], params[2], rsv)
	if err != nil {
		panic(err)
	}
	fmt.Println("txhash", txhash)
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}
func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	if config.FastMPC != nil {
		mpcConfig = mpc.InitConfig(config.FastMPC, true)
	} else {
		mpcConfig = mpc.InitConfig(config.MPC, true)
	}
	log.Info("init config finished", "IsFastMPC", mpcConfig.IsFastMPC)
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[paramChainID]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", paramChainID)
	}
	url = apiAddrs[0]
	apiAddrsExt := cfg.GatewaysExt[paramChainID]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

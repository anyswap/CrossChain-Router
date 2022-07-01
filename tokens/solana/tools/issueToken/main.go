package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
	"github.com/mr-tron/base58"
)

var (
	bridge = solana.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	mpcConfig       *mpc.Config
	chainID         = big.NewInt(0)
)

func main() {

	initAll()

	randomAccount := types.NewAccount()
	fmt.Println(randomAccount.PrivateKey.String())
	fmt.Println(randomAccount.PublicKey().String())

	bs, _ := hex.DecodeString("be0d03d8022012a03d5535e8677681dbbd9bbd130a3593388a61454129f5c294")
	fmt.Println("MPC ADDRESS:", base58.Encode(bs[:]))

	//2MnpF8NgxhvMzH2nCx8MBVKhCSVhiqW63g2xZUQCuic1rUuxir5iJo9eaK4A8BQP5arfkecfLNDcT4QNCxn3o5G9
	//3gScJGwn2GKoi8xjNoSDP6pb9qsnNVAXciWSv7E8yUt5
	mpcAccount, err := types.AccountFromPrivateKeyBase58("2MnpF8NgxhvMzH2nCx8MBVKhCSVhiqW63g2xZUQCuic1rUuxir5iJo9eaK4A8BQP5arfkecfLNDcT4QNCxn3o5G9")
	if err != nil {
		log.Fatal("AccountFromPrivateKeyBase58 err", err)
	}
	mpcPublicKey := mpcAccount.PublicKey().String()
	fmt.Println(mpcPublicKey)
	fmt.Println(hex.EncodeToString(mpcAccount.PublicKey().ToSlice()))

	for i := 0; i < 10; i++ {
		hash, err := bridge.AirDrop(mpcPublicKey, 900000000)
		if err != nil {
			log.Fatal("AirDrop err", err)
		}
		fmt.Println("AirDrop hash: ", hash)
	}

	balance, err := bridge.GetBalance(mpcPublicKey)
	if err != nil {
		log.Fatal("balance err", err)
	}
	fmt.Println("GetBalance : ", balance.String())

	// account, _ := bridge.GetNonceAccountInfo("G3rPGz9Rb1u4uHedg3p5pTfGoSzmj8hxPrJZ7fvFDjnr")
	// fmt.Println("blockhash : ", account.Value.Data.Info.Info.Blockhash)
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

	log.Info("init flags finished")
}

func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	mpcConfig = mpc.InitConfig(config.MPC, true)
	log.Info("init config finished, IsFastMPC: %v", mpcConfig.IsFastMPC)
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

package main

import (
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
	solanatools "github.com/anyswap/CrossChain-Router/v3/tokens/solana/tools"
)

var (
	bridge = solana.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string

	routerProgramID string
	routerMPC       string
	routerPDA       string

	paramNewMpc     string
	paramNewMpcAddr string

	paramPublicKey string
	paramPriKey    string

	err       error
	mpcConfig *mpc.Config
	chainID   = big.NewInt(0)
)

func main() {
	initAll()

	newMpcAddress := ""
	if paramNewMpc == "" {
		newMpcAddress = paramNewMpcAddr
	} else {
		newMpcAddress, err = solana.PublicKeyToAddress(paramNewMpc)
		if err != nil {
			log.Fatalf("NewMpc public key error %v", err)
		}
	}

	fmt.Printf("newMpcAddress: %v\n", newMpcAddress)
	tx, err := bridge.BuildApplyMpcTransaction(routerProgramID, routerMPC, routerPDA, newMpcAddress)
	if err != nil {
		log.Fatal("BuildApplyMpcTransaction err", err)
	}
	signer := &solanatools.Signer{
		PublicKey:  paramPublicKey,
		PrivateKey: paramPriKey,
	}
	txHash := solanatools.SignAndSend(mpcConfig, bridge, []*solanatools.Signer{signer}, tx)

	fmt.Printf("tx success: %v\n", txHash)
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&routerProgramID, "router", "", "router program id")
	flag.StringVar(&routerMPC, "routerMPC", "", "routerMPC address")
	flag.StringVar(&routerPDA, "routerPDA", "", "routerPDA address")

	flag.StringVar(&paramNewMpc, "newMpc", "", "new mpc public key")
	flag.StringVar(&paramNewMpcAddr, "newMpcAddr", "", "new mpc base58 address")

	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")
	flag.StringVar(&paramPriKey, "priKey", "", "signer priKey key")

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
	if config.FastMPC != nil {
		mpcConfig = mpc.InitConfig(config.FastMPC, true)
	} else {
		mpcConfig = mpc.InitConfig(config.MPC, true)
	}
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

package main

import (
	"flag"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
)

var (
	b = cardano.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramFrom       string
	paramTo         string
	paramAsset      string
	paramAmount     string
	bind            string
	toChainId       string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	policy := strings.Split(paramAsset, ".")
	if len(policy) != 2 {
		panic("policy format error")
	}

	// _, _, policyID := b.GetAssetPolicy(paramAsset)
	// assetName := cardanosdk.NewAssetName(paramAsset)
	// assetNameWithPolicy := policyID.String() + "." + common.Bytes2Hex(assetName.Bytes())

	paramAmount, err := common.GetBigIntFromStr(paramAmount)
	if err != nil {
		panic(err)
	}

	fmt.Printf("send asset: %s amount: %d to: %s", paramAsset, paramAmount.Int64(), paramTo)

	utxos, err := b.QueryUtxo(paramFrom, paramAsset, paramAmount)
	if err != nil {
		panic(err)
	}

	swapId := fmt.Sprintf("send_%s_%d", paramAsset, time.Now().Unix())
	rawTx, err := b.BuildTx(swapId, paramTo, paramAsset, paramAmount, utxos)
	if err != nil {
		panic(err)
	}
	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			Identifier: tokens.AggregateIdentifier,
			SwapID:     swapId,
		},
		From:  paramFrom,
		Extra: &tokens.AllExtras{},
	}
	if signTx, _, err := b.MPCSignSwapTransaction(rawTx, args, bind, toChainId); err != nil {
		panic(err)
	} else {
		if txHash, err := b.SendTransaction(signTx); err != nil {
			panic(err)
		} else {
			fmt.Printf("txHash: %s", txHash)
		}
	}

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
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	b.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")

	b.SetChainConfig(&tokens.ChainConfig{
		BlockChain:     "Cardano",
		ChainID:        chainID.String(),
		RouterContract: paramFrom,
		Confirmations:  1,
	})

	b.GetChainConfig().CheckConfig()

	b.InitAfterConfig()

}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramFrom, "from", "", "mpc address")
	flag.StringVar(&paramTo, "to", "", "receive address")
	flag.StringVar(&paramAmount, "amount", "", "receive amount")
	flag.StringVar(&paramAsset, "asset", "", "asset With Policy e.g.f0573f98953b187eec04b21eb25a5983d9d03b0d87223c768555b2ec.55534454")
	flag.StringVar(&bind, "bind", "", "to address")
	flag.StringVar(&toChainId, "toChainId", "", "toChainId")

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

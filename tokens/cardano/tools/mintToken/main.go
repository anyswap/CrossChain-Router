package main

import (
	"flag"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
	cardanosdk "github.com/echovl/cardano-go"
)

var (
	b = cardano.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramFrom       string
	paramTo         string
	paramAsset      string
	paramAmount     string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	_, _, policyID := b.GetAssetPolicy(paramAsset)
	assetName := cardanosdk.NewAssetName(paramAsset)

	paramAmount, err := common.GetBigIntFromStr(paramAmount)
	if err != nil {
		panic(err)
	}

	assetNameWithPolicy := policyID.String() + "." + common.Bytes2Hex(assetName.Bytes())
	fmt.Printf("mint asset: %s amount: %d to: %s", assetNameWithPolicy, paramAmount.Int64(), paramTo)

	utxos, err := b.QueryUtxo(paramFrom, assetNameWithPolicy, paramAmount)
	if err != nil {
		panic(err)
	}

	swapId := fmt.Sprintf("mint_%s_%d", paramAsset, time.Now().Unix())
	rawTx, err := b.BuildTx(swapId, paramTo, assetNameWithPolicy, paramAmount, utxos)
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
	if signTx, _, err := b.MPCSignTransaction(rawTx, args); err != nil {
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

	b.SetChainConfig(&tokens.ChainConfig{
		BlockChain:     "Cardano",
		ChainID:        chainID.String(),
		RouterContract: paramFrom,
		Confirmations:  1,
	})

	b.GetChainConfig().CheckConfig()

	b.InitAfterConfig()
	log.Info("init bridge finished")

}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramFrom, "from", "", "mpc address")
	flag.StringVar(&paramTo, "to", "", "receive address")
	flag.StringVar(&paramAmount, "amount", "", "receive amount")
	flag.StringVar(&paramAsset, "asset", "", "asset eg. USDT")

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

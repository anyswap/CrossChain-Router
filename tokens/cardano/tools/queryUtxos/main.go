package main

import (
	"flag"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
)

var (
	b               = cardano.NewCrossChainBridge()
	paramConfigFile string
	paramChainID    string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config
	paramAddress    string
	paramTxHash     string
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	if res, err := b.GetTransactionByHash(paramTxHash); err != nil {
		log.Fatal("get transaction by txHash error", "txHash", paramTxHash, "err", err)
	} else {
		log.Warnf("transaction:%+v", res)
	}

	if outputs, err := b.GetUtxosByAddress(paramAddress); err != nil {
		log.Fatal("get outputs by address error", "address", paramAddress, "err", err)
	} else {
		log.Warnf("outputs:%+v", outputs)
		utxos := make(map[cardano.UtxoKey]cardano.AssetsMap)
		for _, output := range *outputs {
			utxoKey := cardano.UtxoKey{TxHash: output.TxHash, TxIndex: output.Index}
			utxos[utxoKey] = make(cardano.AssetsMap)

			utxos[utxoKey][cardano.AdaAsset] = output.Value
			for _, token := range output.Tokens {
				utxos[utxoKey][token.Asset.PolicyId+token.Asset.AssetName] = token.Quantity
			}
		}
		log.Warnf("utxos:%+v", utxos)
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
		BlockChain:    "Cardano",
		ChainID:       chainID.String(),
		Confirmations: 1,
	})

	_ = b.GetChainConfig().CheckConfig()

	b.InitAfterConfig()

	log.Info("init bridge finished")
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramAddress, "address", "", "address")
	flag.StringVar(&paramTxHash, "txHash", "", "txHash")
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

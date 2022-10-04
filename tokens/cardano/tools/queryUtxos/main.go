package main

import (
	"flag"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
)

var (
	paramAddress string
	paramTxHash  string
	url          = "https://graphql-api.testnet.dandelion.link/"
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	if res, err := cardano.GetTransactionByHash(url, paramTxHash); err != nil {
		log.Fatal("get transaction by txHash error", "txHash", paramTxHash, "err", err)
	} else {
		log.Warnf("transaction:%+v", res)
	}

	if outputs, err := cardano.GetOutputsByAddress(url, paramAddress); err != nil {
		log.Fatal("get outputs by address error", "address", paramAddress, "err", err)
	} else {
		log.Warnf("outputs:%+v", outputs)
		utxos := make(map[cardano.OutputKey]cardano.UtxoMap)
		for _, output := range *outputs {
			outputKey := cardano.OutputKey{TxHash: output.TxHash, Index: output.Index}
			utxos[outputKey] = cardano.UtxoMap{
				Assets: make(map[string]string),
			}
			utxos[outputKey].Assets[cardano.AdaAsset] = output.Value
			for _, token := range output.Tokens {
				utxos[outputKey].Assets[token.Asset.PolicyId+token.Asset.AssetName] = token.Quantity
			}
		}
		log.Warnf("utxos:%+v", utxos)
	}
}

func initAll() {
	initFlags()
}

func initFlags() {
	flag.StringVar(&paramAddress, "address", "", "address")
	flag.StringVar(&paramTxHash, "txHash", "", "txHash")
	flag.Parse()

	log.Info("init flags finished")
}

package main

import (
	"flag"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet"
	"github.com/dontpanicdao/caigo/types"
)

var (
	paramURL                string
	paramNetwork            string
	paramContractAddress    string
	paramEntrypointSelector string
	paramCalldata           starknet.ArrayFlag
)

var bridge = starknet.NewCrossChainBridge()

func main() {
	initFlags()
	bridge.InitAfterConfig()

	var calldata []string
	for _, s := range paramCalldata {
		calldata = append(calldata, s)
	}

	rawTx, err := bridge.BuildRawInvokeTransaction(paramContractAddress, paramEntrypointSelector, calldata...)
	if err != nil {
		log.Error("build raw invoke tx failed", err)
	}
	signedTx, _, err := bridge.SignTransactionWithPrivateKey(starknet.EC256STARK, rawTx, "")
	if err != nil {
		log.Error("sign with private key failed", err)
	}
	txHash, err := bridge.SendTransaction(signedTx)
	if err != nil {
		log.Error("send tx failed", err)
	}
	state, err := bridge.WaitForTransaction(types.HexToHash(txHash), 10)
	if err != nil {
		log.Error("wait for tx failed", err)
	}
	log.Info("tx state: ", state)
	_, err = bridge.GetTransaction(txHash)
	if err != nil {
		log.Error("wait for tx failed", err)
	}
}

func initFlags() {
	flag.StringVar(&paramURL, "url", "", "rpc url")
	flag.StringVar(&paramNetwork, "network", "", "mainnet or testnet")
	flag.StringVar(&paramContractAddress, "contract address", "", "contract address")
	flag.StringVar(&paramEntrypointSelector, "entrypoint selector", "", "entrypoint selector, i.e. function name")
	flag.Var(&paramCalldata, "calldata", "calldata")
	flag.Parse()
}

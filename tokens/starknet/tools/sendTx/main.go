package main

import (
	"flag"
	"math/big"

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

func main() {
	initFlags()
	bridge := starknet.NewCrossChainBridge(new(big.Int).SetUint64(1546503017845)) // testnet
	bridge.InitAfterConfig()

	var calldata []string
	for _, s := range paramCalldata {
		calldata = append(calldata, s)
	}

	call := bridge.PrepFunctionCall(paramContractAddress, paramEntrypointSelector, calldata)
	rawInvokeTx, err := bridge.BuildRawInvokeTx(call, nil)
	if err != nil {
		log.Error("build raw invoke tx failed", err)
	}
	signedTx, _, err := bridge.SignTransactionWithPrivateKey(starknet.EC256STARK, rawInvokeTx, "")
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

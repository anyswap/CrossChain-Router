package main

import (
	"flag"
	"fmt"

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

	var calldata []string
	for _, s := range paramCalldata {
		calldata = append(calldata, s)
	}

	c := types.FunctionCall{
		ContractAddress:    types.HexToHash(paramContractAddress),
		EntryPointSelector: paramEntrypointSelector,
		Calldata:           calldata,
	}
	provider, err := starknet.NewProvider(paramURL, starknet.GetStubChainID(paramNetwork))
	if err != nil {
		log.Error("make new provider failed ", err)
	}

	ret, err := provider.Call(c)
	if err != nil {
		log.Error("call failed ", err)
	}

	fmt.Println(ret)
}

func initFlags() {
	flag.StringVar(&paramURL, "url", "", "rpc url")
	flag.StringVar(&paramNetwork, "network", "", "mainnet or testnet")
	flag.StringVar(&paramContractAddress, "contract address", "", "contract address")
	flag.StringVar(&paramEntrypointSelector, "entrypoint selector", "", "entrypoint selector, i.e. function name")
	flag.Var(&paramCalldata, "calldata", "calldata")
	flag.Parse()
}

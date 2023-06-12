package main

import (
	"fmt"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet"
	ctypes "github.com/dontpanicdao/caigo/types"
	"log"
	"math/big"
	"time"
)

const (
	invokeType   = "invoke"
	estimateType = "estimate"
	mpcAddress   = "0x05438a440ee7043444875c89f1d8983ad57bdef93b8a6b32df82ce4b6d141ebd"
	//mpcAddress = "0x06aed585647bb16e2444c32a4465c361c9b1928d04df6c2daa62a911630a683d"
	mpcPubKey = "04e8082be957ea371256cf8477927d87d77889ae00d9a905a84e65c5db699d351ba49bc3db59f8b95c052e62eb2c0a5627c08e4421cc6120240be0aa82c2695c2a"
)

var chainIDTestnet = new(big.Int).SetUint64(1546503017845)
var url = "https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f" // RPC url
var bridge *starknet.Bridge
var args = &tokens.BuildTxArgs{}

var (
	routerContract  = "0x07a9871c19c588edb30b6614c38d48584b4ca027e1d044d77d8a2550319fdfb3"
	selector        = "anySwapOut"
	swapOutCalldata = []string{"0x01ba8f685d61529f9613febe7b41b438b6b4f4aa163ff5b94427e8ab4cfc4050", "0x906431cf4c398821da457B13A6fCeD3f01Ee1d2b", "100000", "0", "5"}
)

func init() {
	Config = mpc.InitConfig(mpcConfig, true)
	bridge = starknet.NewCrossChainBridge(chainIDTestnet)
	bridge.ChainConfig = &tokens.ChainConfig{Extra: mpcAddress}
	bridge.GatewayConfig = &tokens.GatewayConfig{AllGatewayURLs: []string{url}}
	bridge.InitAfterConfig()
}

func main() {
	call := bridge.PrepFunctionCall(routerContract, selector, swapOutCalldata)

	// estimate fee
	estimateTx, _ := buildInvokeTx(call, estimateType, nil)
	fee, err := bridge.EstimateFee(estimateTx)
	if err != nil {
		log.Fatal("send estimate failed: ", err)
	}
	fmt.Println("estimate fee: ", fee.String())

	// invoke
	invokeTx, _ := buildInvokeTx(call, invokeType, fee)
	txHash, err := bridge.SendTransaction(invokeTx)
	if err != nil {
		log.Fatal("send invoke failed: ", err)
	}
	fmt.Println("tx hash: ", txHash)

	//  (optional) wait for tx
	txSent := time.Now()
	state, err := bridge.WaitForTransaction(ctypes.HexToHash(txHash), 10)
	if err != nil {
		log.Fatal("could not execute transaction: ", err)
	}
	fmt.Println("state: ", state)
	fmt.Println("time (seconds) since tx sent: ", time.Since(txSent).Seconds())
}

func buildInvokeTx(call starknet.FunctionCall, callType string, fee *big.Int) (interface{}, error) {
	// 1. build execute details
	details, hash, err := bridge.PrepExecDetails(call, callType, fee, args)
	if err != nil {
		log.Fatal("build upper failed: ", err)
	}

	// 2. mpc sign
	keyID, rsvs, err := Config.DoSignOneEC(mpcPubkey, hash, "swapOut estimate")
	if err != nil {
		log.Fatal("sign failed: ", err)
	}
	fmt.Printf("[estimate] KeyID: %s\n", keyID)

	// 3. build signed invoke tx
	return bridge.PrepSignedInvokeTx(rsvs[0], call, details)
}

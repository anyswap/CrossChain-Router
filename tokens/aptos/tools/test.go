package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
)

func main() {
	client.InitHTTPClient()
	restClient := aptos.RestClient{
		Url:     "http://fullnode.devnet.aptoslabs.com",
		Timeout: 10000,
	}

	// 0x765145134cbec9c27e8f49e6bb93742884db0ca59bad824d84e50a8cc12d958d
	alice := aptos.NewAccountFromSeed("a5dcf0efd8264e4c6bfb3699facf80128e83bd6c4955ca14c243550c9c7863e8")

	// 0x8d317935e44cdee5e3f70128e38312eb58925466cb92ba09b1020ee929267381
	bob := aptos.NewAccountFromSeed("579be509353d74a44e3d57cfb985b1f5de74a9f972586d71b1d93d2d556a0ed8")

	UnderlyingModule := "TestCoin"
	underlyingCoinStruct := fmt.Sprintf("%s::%s::MyCoin", bob.GetHexAddress(), UnderlyingModule)
	PoolCoinModule := "PoolCoin"
	poolcoinStuct := fmt.Sprintf("%s::%s::AnyMyCoin", alice.GetHexAddress(), PoolCoinModule)

	resp0, _ := restClient.GetLedger()
	println(resp0.ChainId, resp0.LedgerTimestamp)

	resp, _ := restClient.GetAccount(alice.GetHexAddress())
	println(resp.AuthenticationKey, resp.SequenceNumber)

	// resp1, _ := restClient.GetAccountCoin(alice.GetHexAddress(), aptos.NATIVE_COIN)
	// println(resp1.Data.Coin.Value)

	// resp2, _ := restClient.GetAccountResource(bob.GetHexAddress(), "0x1::coin::CoinInfo<0x8d317935e44cdee5e3f70128e38312eb58925466cb92ba09b1020ee929267381::TestCoin::MyCoin>")
	// println(resp2.Data.Name, resp2.Data.Symbol, resp2.Data.Decimals)

	timeout := time.Now().Unix() + 600
	// requestBody := aptos.Transaction{
	// 	Sender:                  alice.GetHexAddress(),
	// 	SequenceNumber:          resp.SequenceNumber,
	// 	MaxGasAmount:            "1000",
	// 	GasUnitPrice:            "1",
	// 	ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
	// 	Payload: &aptos.TransactionPayload{
	// 		Type:          aptos.SCRIPT_FUNCTION_PAYLOAD,
	// 		Function:      "0x1::coin::transfer",
	// 		TypeArguments: []string{aptos.NATIVE_COIN},
	// 		Arguments:     []string{bob.GetHexAddress(), "10"},
	// 	},
	// }
	requestBody := aptos.Transaction{
		Sender:                  alice.GetHexAddress(),
		SequenceNumber:          resp.SequenceNumber,
		MaxGasAmount:            "1000",
		GasUnitPrice:            "1",
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &aptos.TransactionPayload{
			Type:          aptos.SCRIPT_FUNCTION_PAYLOAD,
			Function:      aptos.GetRouterFunctionId(alice.GetHexAddress(), aptos.CONTRACT_NAME, aptos.CONTRACT_FUNC_SWAPIN),
			TypeArguments: []string{underlyingCoinStruct, poolcoinStuct},
			Arguments:     []string{bob.GetHexAddress(), strconv.FormatUint(100, 10), "0x746573742121", "5777"},
		},
	}
	resp4, err := restClient.GetSigningMessage(&requestBody)
	if err != nil {
		log.Fatal("GetSigningMessage", "err", err)
	}
	println(*resp4)

	// ed25519

	signature, err := alice.SignString(*resp4)
	if err != nil {
		log.Fatal("SignString", "err", err)
	}
	ts := aptos.TransactionSignature{
		Type:      "ed25519_signature",
		PublicKey: alice.GetPublicKeyHex(),
		Signature: signature,
	}

	requestBody.Signature = &ts

	resp5, err := restClient.SubmitTranscation(&requestBody)
	if err != nil {
		log.Fatal("SubmitTranscation", "err", err)
	}
	println(resp5.Hash)

	resp3, _ := restClient.GetTransactions(resp5.Hash)
	jsonData, _ := json.Marshal(resp3.Events)
	println(resp3.Success, resp3.Sender, string(jsonData))

	resp1, _ := restClient.GetAccountCoin(bob.GetHexAddress(), poolcoinStuct)
	println(resp1.Data.Coin.Value)

}

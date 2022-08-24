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

var (
	UnderlyingModule = "TestCoin"
	PoolCoinModule   = "PoolCoin"
)

func main() {
	client.InitHTTPClient()
	restClient := aptos.RestClient{
		Url:     "http://fullnode.devnet.aptoslabs.com",
		Timeout: 10000,
	}

	// 0xc441fa1354b4544457df58b7bfdf53fae75e0d6f61ded55b72ae058d2d407c9d
	alice := aptos.NewAccountFromSeed("d19306e8258f44191646999a90e0066e142122944d0ccc8c4858d9f991ded5b3")

	// 0x27b1c07abb2146204ba281464ace56075c7d1338a8df0fbe44245674b6fa1309
	bob := aptos.NewAccountFromSeed("67dbdec0a79101c8d61c4a55785923b20c0ef9a5886a79cec431d9216d24f576")

	// resp0, _ := restClient.GetLedger()
	// println(resp0.ChainId, resp0.LedgerTimestamp)

	// resp1, _ := restClient.GetAccountCoin(alice.GetHexAddress(), aptos.NATIVE_COIN)
	// println(resp1.Data.Coin.Value)

	// resp2, _ := restClient.GetAccountResource(bob.GetHexAddress(), "0x1::coin::CoinInfo<0x8d317935e44cdee5e3f70128e38312eb58925466cb92ba09b1020ee929267381::TestCoin::MyCoin>")
	// println(resp2.Data.Name, resp2.Data.Symbol, resp2.Data.Decimals)

	Transfer(alice, bob, &restClient, 10)

	Swapin(alice, bob, &restClient, 100000000000)

	Swapout(alice, bob, &restClient, 100000000000)

}

func Transfer(alice, bob *aptos.Account, restClient *aptos.RestClient, amount uint64) {
	resp, _ := restClient.GetAccount(alice.GetHexAddress())
	println(resp.AuthenticationKey, resp.SequenceNumber)

	timeout := time.Now().Unix() + 600
	requestBody := aptos.Transaction{
		Sender:                  alice.GetHexAddress(),
		SequenceNumber:          resp.SequenceNumber,
		MaxGasAmount:            "1000",
		GasUnitPrice:            "1",
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &aptos.TransactionPayload{
			Type:          aptos.SCRIPT_FUNCTION_PAYLOAD,
			Function:      "0x1::coin::transfer",
			TypeArguments: []string{aptos.NATIVE_COIN},
			Arguments:     []string{bob.GetHexAddress(), strconv.FormatUint(amount, 10)},
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
}

func Swapin(alice, bob *aptos.Account, restClient *aptos.RestClient, amount uint64) {
	underlyingCoinStruct := fmt.Sprintf("%s::%s::MyCoin", bob.GetHexAddress(), UnderlyingModule)
	poolcoinStuct := fmt.Sprintf("%s::%s::AnyMyCoin", alice.GetHexAddress(), PoolCoinModule)
	resp, _ := restClient.GetAccount(alice.GetHexAddress())
	println(resp.AuthenticationKey, resp.SequenceNumber)

	timeout := time.Now().Unix() + 600
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
			Arguments:     []string{bob.GetHexAddress(), strconv.FormatUint(amount, 10), "0x1234567890123456789012345678901234567890", "5777"},
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

func Swapout(alice, bob *aptos.Account, restClient *aptos.RestClient, amount uint64) {
	poolcoinStuct := fmt.Sprintf("%s::%s::AnyMyCoin", alice.GetHexAddress(), PoolCoinModule)
	resp, _ := restClient.GetAccount(bob.GetHexAddress())
	println(resp.AuthenticationKey, resp.SequenceNumber)

	timeout := time.Now().Unix() + 600
	requestBody := aptos.Transaction{
		Sender:                  bob.GetHexAddress(),
		SequenceNumber:          resp.SequenceNumber,
		MaxGasAmount:            "1000",
		GasUnitPrice:            "1",
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &aptos.TransactionPayload{
			Type:          aptos.SCRIPT_FUNCTION_PAYLOAD,
			Function:      aptos.GetRouterFunctionId(alice.GetHexAddress(), aptos.CONTRACT_NAME, aptos.CONTRACT_FUNC_SWAPOUT),
			TypeArguments: []string{poolcoinStuct},
			Arguments:     []string{strconv.FormatUint(amount, 10), "0xC5107334A3Ae117E3DaD3570b419618C905Aa5eC", "5777"},
		},
	}
	resp4, err := restClient.GetSigningMessage(&requestBody)
	if err != nil {
		log.Fatal("GetSigningMessage", "err", err)
	}
	println(*resp4)

	// ed25519

	signature, err := bob.SignString(*resp4)
	if err != nil {
		log.Fatal("SignString", "err", err)
	}
	ts := aptos.TransactionSignature{
		Type:      "ed25519_signature",
		PublicKey: bob.GetPublicKeyHex(),
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

package main

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
)

func main() {
	client.InitHTTPClient()
	restClient := aptos.RestClient{
		Url:     "http://fullnode.devnet.aptoslabs.com",
		Timeout: 10000,
	}

	// resp0, _ := restClient.GetLedger()
	// println(resp0.ChainId, resp0.LedgerTimestamp)

	// resp, _ := restClient.GetAccount("0x1006a78099e019ca767bc617ec4e865149148bfebc944d2398f8c0d178931827")
	// println(resp.AuthenticationKey, resp.SequenceNumber)

	// resp1, _ := restClient.GetAccountCoinStore("0x1006a78099e019ca767bc617ec4e865149148bfebc944d2398f8c0d178931827", "0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>")
	// println(resp1.Data.Coin.Value)

	// resp2, _ := restClient.GetAccountCoin("0x1006a78099e019ca767bc617ec4e865149148bfebc944d2398f8c0d178931827", "0x1::coin::CoinInfo<0x1006a78099e019ca767bc617ec4e865149148bfebc944d2398f8c0d178931827::PoolCoin::AnyMyCoin>")
	// println(resp2.Data.Name, resp2.Data.Symbol, resp2.Data.Decimals)

	// resp3, _ := restClient.GetTransactions("0x4bd7e2cd476af7046c2c496d2c6f06c4045f82f7b0ce83293c837d818d35cd80")
	// println(resp3.Success, resp3.Sender)

	body := `{"sender":"0x5ab890371ff7244913a2941098ba3238c9621d50ba2c80fbfa611a9e18c285b1","sequence_number":"1","max_gas_amount":"5000","gas_unit_price":"1","expiration_timestamp_secs":"1660276227","payload":{"type":"script_function_payload","function":"0x1::coin::transfer","type_arguments":["0x1::aptos_coin::AptosCoin"],"arguments":["0xcd5e3e73791ec5c7e3fe4a79448bda8aa6e00c49071d00562362a672a65e835b","1000"]}}`
	requestBody := aptos.Transaction{}
	err := json.Unmarshal([]byte(body), &requestBody)
	if err != nil {
		log.Fatal("Unmarshal", "err", err)
	}
	timeout := time.Now().Unix() + 600
	requestBody.ExpirationTimestampSecs = strconv.FormatInt(timeout, 10)
	resp4, err := restClient.GetSigningMessage(&requestBody)
	if err != nil {
		log.Fatal("GetSigningMessage", "err", err)
	}
	println(resp4.Message)

	// keypair, err := tweetnacl.CryptoSignKeyPair()
	// if err != nil {
	// 	log.Fatal("CryptoSignKeyPair", "err", err)
	// }
	// println(common.ToHex(keypair.PublicKey), common.ToHex(keypair.SecretKey))

	account := aptos.NewAccountFromSeed("56b389a1b1f34d49ed196c6459d8b57768cf8ef32f34b3110e45025cd26a2e58")
	// Alice: 0x818896432a3b80529587f2ee5239e4fe8fb259bb53ca8311fcc5762fc48c8ad4 PukKey:b006c46f4c239c8eae05b50d9e4d8acf66e4ae802e7098fc2f8608444b4e6af9 Key Seed: 7d7a87455d93ad9f915319948dde4368c34fb0f04fa6efcfba8317f671bd8ac5
	// Bob: 0x47195e866527a0286908e0154b2b2698f4a3f418a1057369698578b3d9a464af PukKey:b006c46f4c239c8eae05b50d9e4d8acf66e4ae802e7098fc2f8608444b4e6af9 Key Seed: 858f68c24b0778fabe9b991cd91f7c933a7ce4d0d855716583b22c33a6f2743b
	println(common.ToHex(account.KeyPair.PublicKey), common.ToHex(account.KeyPair.SecretKey), account.GetHexAddress())

	// account1 := aptos.NewAccountFromPubkey("b006c46f4c239c8eae05b50d9e4d8acf66e4ae802e7098fc2f8608444b4e6af9")
	// println(account1.GetHexAddress())

	signature, err := account.SignString(resp4.Message)
	if err != nil {
		log.Fatal("SignString", "err", err)
	}
	ts := aptos.TransactionSignature{
		Type:      "ed25519_signature",
		PublicKey: account.GetPublicKeyHex(),
		Signature: signature,
	}

	requestBody.Signature = &ts

	resp5, err := restClient.SubmitTranscation(&requestBody)
	if err != nil {
		log.Fatal("SubmitTranscation", "err", err)
	}
	println(resp5.Hash)
}

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
)

var (
	bridge = aptos.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string

	paramPublicKey string
	paramPriKey    string

	coin      string
	toAddress string
	toChainId string
	amount    uint64

	mpcConfig *mpc.Config
)

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")
	flag.StringVar(&paramPriKey, "priKey", "", "signer priKey key")

	flag.StringVar(&coin, "coin", "", "coin resource")
	flag.StringVar(&toAddress, "toAddress", "", "toAddress")
	flag.StringVar(&toChainId, "toChainId", "", "toChainId")
	flag.Uint64Var(&amount, "amount", 0, "anycoin name")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)
	initAll()

	var account *aptos.Account
	if paramPriKey != "" {
		account = aptos.NewAccountFromSeed(paramPriKey)
	} else {
		account = aptos.NewAccountFromPubkey(paramPublicKey)
	}
	log.Info("SignAccount", "address", account.GetHexAddress())
	tx, err := bridge.BuildSwapoutTransaction(account.GetHexAddress(), coin, toAddress, toChainId, amount)
	if err != nil {
		log.Fatalf("%v", err)
	}
	signingMessage, err := bridge.Client.GetSigningMessage(tx)
	if err != nil {
		log.Fatal("GetSigningMessage", "err", err)
	}
	if paramPriKey != "" {
		signature, err := account.SignString(*signingMessage)
		if err != nil {
			log.Fatal("SignString", "err", err)
		}
		tx.Signature = &aptos.TransactionSignature{
			Type:      "ed25519_signature",
			PublicKey: account.GetPublicKeyHex(),
			Signature: signature,
		}
		log.Info("SignTransactionWithPrivateKey", "signature", signature)

	} else {
		mpcPubkey := paramPublicKey

		msgContent := *signingMessage
		jsondata, _ := json.Marshal(tx)
		msgContext := string(jsondata)

		keyID, rsvs, err := mpcConfig.DoSignOneED(mpcPubkey, msgContent, msgContext)
		if err != nil {
			log.Fatal("DoSignOneED", "err", err)
		}
		log.Info("DoSignOneED", "keyID", keyID)

		if len(rsvs) != 1 {
			log.Fatal("DoSignOneED", "err", errors.New("get sign status require one rsv but return many"))
		}
		rsv := rsvs[0]
		tx.Signature = &aptos.TransactionSignature{
			Type:      "ed25519_signature",
			PublicKey: mpcPubkey,
			Signature: rsv,
		}
		log.Info("DoSignOneED", "signature", rsv)
	}
	txInfo, err := bridge.Client.SubmitTranscation(tx)
	if err != nil {
		log.Fatal("SignString", "err", err)
	}
	time.Sleep(time.Duration(10) * time.Second)
	result, _ := bridge.Client.GetTransactions(txInfo.Hash)
	log.Info("SubmitTranscation", "txHash", txInfo.Hash, "Success", result.Success, "version", result.Version, "vm_status", result.VmStatus)
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
	apiAddrs := cfg.Gateways[paramChainID]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", paramChainID)
	}
	apiAddrsExt := cfg.GatewaysExt[paramChainID]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

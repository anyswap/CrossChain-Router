package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
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

	paramPath    string
	paramModules string

	mpcConfig *mpc.Config
)

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")
	flag.StringVar(&paramPriKey, "priKey", "", "signer priKey key")

	flag.StringVar(&paramPath, "path", "", "contract build path")
	flag.StringVar(&paramModules, "modules", "", "deploy module name,split with ','")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)
	initAll()
	if paramPath == "" {
		log.Fatal("path can't be empty")
	}

	packageMetaData := readMove(paramPath + "/package-metadata.bcs")
	// fmt.Println("packageMetaData", packageMetaData)

	moduleArray := strings.Split(paramModules, ",")
	moveHexs := []string{}
	for _, moduleName := range moduleArray {
		moveHex := readMove(paramPath + "/bytecode_modules/" + moduleName + ".mv")
		// fmt.Println(moveHex)
		moveHexs = append(moveHexs, moveHex)
	}

	var account *aptos.Account
	if paramPriKey != "" {
		account = aptos.NewAccountFromSeed(paramPriKey)
	} else {
		account = aptos.NewAccountFromPubkey(paramPublicKey)
	}

	tx, err := bridge.BuildDeployModuleTransaction(account.GetHexAddress(), packageMetaData, moveHexs)
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
		log.Fatal("SubmitTranscation", "err", err)
	}

	result, err := bridge.Client.GetTransactionsNotPending(txInfo.Hash)
	if err != nil {
		log.Fatal("GetTransactionsNotPending", "err", err)
	}
	log.Info("SubmitTranscation", "txHash", txInfo.Hash, "Success", result.Success)
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

func readMove(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("readMove", "filename", filename)
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatal("ReadAll", "filename", filename)
	}
	return common.ToHex(content)
}

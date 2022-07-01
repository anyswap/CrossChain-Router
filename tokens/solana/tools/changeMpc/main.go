package main

import (
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

var (
	bridge = solana.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramNewMpc     string
	paramNewMpcAddr string

	paramPublicKey string
	paramPriKey    string

	err       error
	mpcConfig *mpc.Config
	chainID   = big.NewInt(0)
)

func main() {
	initAll()

	newMpcAddress := ""
	if paramNewMpc == "" {
		newMpcAddress = paramNewMpcAddr
	} else {
		newMpcAddress, err = solana.PublicKeyToAddress(paramNewMpc)
		if err != nil {
			log.Fatalf("NewMpc public key error %v", err)
		}
	}

	fmt.Printf("newMpcAddress: %v\n", newMpcAddress)
	tx, err := bridge.BuildChangeMpcTransaction(newMpcAddress)
	if err != nil {
		log.Fatal("BuildChangeMpcTransaction err", err)
	}
	signerKeys := tx.Message.SignerKeys()
	if len(signerKeys) != 1 {
		log.Fatal("wrong number of signer keys", err)
	}

	var txHash string
	if paramPriKey != "" {
		_, txHash, err = bridge.SignTransactionWithPrivateKey(tx, paramPriKey)
		if err != nil {
			log.Fatal("SignTransactionWithPrivateKey err", err)
		}
	} else {
		var keyID string
		var rsvs []string
		msgContent, err := tx.Message.Serialize()
		if err != nil {
			log.Fatal("unable to encode message for signing", err)
		}
		keyID, rsvs, err = mpcConfig.DoSignOneED(paramPublicKey, common.ToHex(msgContent[:]), "solanaChangeMPC")
		if len(rsvs) != 1 {
			log.Fatal("get sign status require one rsv but return many", err)
		}
		rsv := rsvs[0]
		sig, err := types.NewSignatureFromString(rsv)
		if err != nil {
			log.Fatal("get signature from rsv failed", "keyID", keyID, "txid", err)
		}

		tx.Signatures = append(tx.Signatures, sig)
		txHash = sig.String()
	}

	sendTxHash, err := bridge.SendTransaction(tx)
	if err != nil {
		log.Fatal("SendTransaction err", err)
	}

	if sendTxHash != txHash {
		log.Fatal("SendTransaction sendTxHash not same")
	}

	fmt.Printf("tx success: %v\n", sendTxHash)
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramNewMpc, "newMpc", "", "new mpc public key")
	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")
	flag.StringVar(&paramPriKey, "priKey", "", "signer priKey key")

	flag.StringVar(&paramNewMpcAddr, "newMpcAddr", "", "new mpc base58 address")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

	log.Info("init flags finished")
}

func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	if config.FastMPC != nil {
		mpcConfig = mpc.InitConfig(config.FastMPC, true)
	} else {
		mpcConfig = mpc.InitConfig(config.MPC, true)
	}
	log.Info("init config finished, IsFastMPC: %v", mpcConfig.IsFastMPC)
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

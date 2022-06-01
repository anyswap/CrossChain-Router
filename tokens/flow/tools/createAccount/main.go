package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/flow"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access/http"
	fcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
	"github.com/onflow/flow-go-sdk/templates"
)

var (
	bridge = flow.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramAddress    string
	paramPublicKey  string
	paramPrivKey    string
	paramNewPrivKey string
	chainID                = big.NewInt(0)
	defaultGasLimit uint64 = 1_000_000
	ctx                    = context.Background()
	mpcConfig       *mpc.Config
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	err := checkParams()
	if err != nil {
		log.Fatal("checkParams failed", "err", err)
	}

	url := bridge.GatewayConfig.APIAddress[0]
	flowClient, err := http.NewClient(url)
	if err != nil {
		log.Fatal("connect failed", "url", url, "err", err)
	}

	newPrivKey, err := fcrypto.DecodePrivateKeyHex(fcrypto.ECDSA_secp256k1, paramNewPrivKey)
	if err != nil {
		log.Fatal("DecodePrivateKeyHex failed", "paramNewPrivKey", paramNewPrivKey, "err", err)
	}

	myAcctKey := sdk.NewAccountKey().
		FromPrivateKey(newPrivKey).
		SetHashAlgo(fcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	referenceBlockID := examples.GetReferenceBlockId(flowClient)
	payerAddress := sdk.HexToAddress(paramAddress)

	index, err := bridge.GetAccountIndex(paramAddress)
	if err != nil {
		log.Fatal("GetAccountIndex failed", "payerAddress", payerAddress, "err", err)
	}

	sequenceNumber, err := bridge.GetAccountNonce(paramAddress)
	if err != nil {
		log.Fatal("GetAccountNonce failed", "payerAddress", payerAddress, "err", err)
	}

	createAccountTx, err := templates.CreateAccount([]*sdk.AccountKey{myAcctKey}, nil, payerAddress)
	if err != nil {
		log.Fatal("CreateAccount failed", "payerAddress", payerAddress, "err", err)
	}

	createAccountTx.SetProposalKey(
		payerAddress,
		index,
		sequenceNumber,
	)

	createAccountTx.SetReferenceBlockID(referenceBlockID)
	createAccountTx.SetPayer(payerAddress)

	if paramPrivKey != "" {
		ecPrikey, err := fcrypto.DecodePrivateKeyHex(fcrypto.ECDSA_P256, paramPrivKey)
		if err != nil {
			log.Fatal("DecodePrivateKeyHex failed", "privKey", paramPrivKey, "err", err)
		}

		keySigner, err := fcrypto.NewInMemorySigner(ecPrikey, fcrypto.SHA3_256)
		if err != nil {
			log.Fatal("NewInMemorySigner failed", "ecPrikey", ecPrikey, "err", err)
		}

		err = createAccountTx.SignEnvelope(payerAddress, index, keySigner)
		if err != nil {
			log.Fatal("SignEnvelope failed", "payerAddress", payerAddress, "index", index, "err", err)
		}

		err = flowClient.SendTransaction(ctx, *createAccountTx)
		if err != nil {
			log.Fatal("SendTransaction failed", "createAccountTx", createAccountTx, "index", index, "err", err)
		}

		fmt.Printf("SendTransaction success,txHash:%+v", createAccountTx.ID().Hex())
		return
	}

	MPCSignTransaction(createAccountTx, paramPublicKey)
}

func MPCSignTransaction(rawTx interface{}, paramPublicKey string) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*sdk.Transaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}
	message := tx.EnvelopeMessage()
	message = append(sdk.TransactionDomainTag[:], message...)

	keyID, rsvs, err := mpcConfig.DoSignOneEC(paramPublicKey, common.ToHex(message[:]), "")
	if err != nil {
		return nil, "", err
	}

	if len(rsvs) != 1 {
		log.Warn("get sign status require one rsv but return many",
			"rsvs", len(rsvs), "keyID", keyID)
		return nil, "", errors.New("get sign status require one rsv but return many")
	}

	rsv := rsvs[0]
	sig := common.FromHex(rsv)
	if len(sig) != crypto.SignatureLength {
		log.Error("wrong signature length", "keyID", keyID, "have", len(sig), "want", crypto.SignatureLength)
		return nil, "", errors.New("wrong signature length")
	}

	tx.AddEnvelopeSignature(tx.Payer, tx.ProposalKey.KeyIndex, sig)

	txHash = tx.ID().String()
	log.Info("success", "keyID", keyID, "txhash", txHash, "nonce", tx.ProposalKey.SequenceNumber)
	return tx, txHash, err
}

func checkParams() error {
	err := bridge.VerifyPubKey(paramAddress, paramPublicKey)
	if err != nil {
		return err
	}

	return nil
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramAddress, "address", "", "signer address")
	flag.StringVar(&paramPublicKey, "pubKey", "", "signer public key")
	flag.StringVar(&paramPrivKey, "privKey", "", "(option) signer paramPrivKey key")
	flag.StringVar(&paramNewPrivKey, "newPrivKey", "", "new key privKey address")

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
	mpcConfig = mpc.InitConfig(config.MPC, true)
	log.Info("init config finished")
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

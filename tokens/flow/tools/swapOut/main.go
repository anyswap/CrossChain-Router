package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/flow"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access/grpc"
	fcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
)

var (
	bridge = flow.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramAddress    string
	paramPublicKey  string
	paramPrivKey    string
	paramToken      string
	paramTo         string
	paramToChainID  string
	paramValue      string
	chainID         = big.NewInt(0)
	ctx             = context.Background()
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
	flowClient, err := grpc.NewClient(url)
	if err != nil {
		log.Fatal("connect failed", "url", url, "err", err)
	}

	referenceBlockID := examples.GetReferenceBlockId(flowClient)
	payerAddress := sdk.HexToAddress(paramAddress)

	index, err := bridge.GetAccountIndex(paramAddress, paramPublicKey)
	if err != nil {
		log.Fatal("GetAccountIndex failed", "payerAddress", payerAddress, "err", err)
	}

	sequenceNumber, err := bridge.GetAccountNonce(paramAddress, paramPublicKey)
	if err != nil {
		log.Fatal("GetAccountNonce failed", "payerAddress", payerAddress, "err", err)
	}

	initScript, errf := ioutil.ReadFile("tokens/flow/transaction/SwapOut.cdc")
	if errf != nil {
		log.Fatal("ReadFile failed", "errf", errf)
	}
	script := string(initScript)
	script = fmt.Sprintf(script, paramAddress, paramAddress)

	tx := sdk.NewTransaction().
		SetScript([]byte(script)).
		SetReferenceBlockID(referenceBlockID).
		SetProposalKey(payerAddress, index, sequenceNumber).
		SetPayer(payerAddress).
		AddAuthorizer(payerAddress)

	tokenInditifier := cadence.String(paramToken)
	bindAddr := cadence.String(paramTo)
	id, err := common.GetUint64FromStr(paramToChainID)
	if err != nil {
		log.Fatal("build tx fails", "err", err)
	}
	toChainID := cadence.NewUInt64(id)

	value, err := cadence.NewUFix64(paramValue)
	if err != nil {
		log.Fatal("build tx fails", "err", err)
	}
	err = tx.AddArgument(tokenInditifier)
	if err != nil {
		log.Fatal("build tx fails", "err", err)
	}
	err = tx.AddArgument(bindAddr)
	if err != nil {
		log.Fatal("build tx fails", "err", err)
	}
	err = tx.AddArgument(toChainID)
	if err != nil {
		log.Fatal("build tx fails", "err", err)
	}
	err = tx.AddArgument(value)
	if err != nil {
		log.Fatal("build tx fails", "err", err)
	}

	if paramPrivKey != "" {
		ecPrikey, err := fcrypto.DecodePrivateKeyHex(fcrypto.ECDSA_secp256k1, paramPrivKey)
		if err != nil {
			log.Fatal("DecodePrivateKeyHex failed", "privKey", paramPrivKey, "err", err)
		}

		keySigner, err := fcrypto.NewInMemorySigner(ecPrikey, fcrypto.SHA3_256)
		if err != nil {
			log.Fatal("NewInMemorySigner failed", "ecPrikey", ecPrikey, "err", err)
		}

		err = tx.SignEnvelope(payerAddress, index, keySigner)
		if err != nil {
			log.Fatal("SignEnvelope failed", "payerAddress", payerAddress, "index", index, "err", err)
		}

		err = flowClient.SendTransaction(ctx, *tx)
		if err != nil {
			log.Fatal("SendTransaction failed", "createAccountTx", tx, "index", index, "err", err)
		}

		log.Info("SendTransaction success", "hash", tx.ID().Hex())
		return
	}

	signedTx, txHash, err := MPCSignTransaction(tx, paramPublicKey)
	if err != nil {
		log.Fatal("MPCSignTransaction failed", "paramPublicKey", paramPublicKey)
	}
	log.Info("sign tx success", "hash", txHash)

	// send tx
	txHash, err = bridge.SendTransaction(signedTx)
	if err != nil {
		log.Fatal("SendTransaction failed", "signedTx", signedTx)
	}
	log.Info("SendTransaction success", "hash", txHash)

}

func MPCSignTransaction(rawTx interface{}, paramPublicKey string) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*sdk.Transaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}
	message := tx.EnvelopeMessage()
	message = append(sdk.TransactionDomainTag[:], message...)
	hasher, _ := fcrypto.NewHasher(fcrypto.SHA3_256)
	hash := hasher.ComputeHash(message)
	mpcRealPubkey, err := bridge.PubKeyToMpcPubKey(paramPublicKey)
	if err != nil {
		return nil, "", err
	}
	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcRealPubkey, common.ToHex(hash[:]), "")
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

	tx.AddEnvelopeSignature(tx.Payer, tx.ProposalKey.KeyIndex, sig[:64])

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
	flag.StringVar(&paramToken, "token", "", "swap out token identifier")
	flag.StringVar(&paramTo, "to", "", "dest chain receiver addr")
	flag.StringVar(&paramToChainID, "toChainId", "", "dest chain id")
	flag.StringVar(&paramValue, "value", "", "swap out amount")

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
	if paramPrivKey == "" {
		mpcConfig = mpc.InitConfig(config.MPC, true)
	}
	log.Info("init config finished", "config", config)
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

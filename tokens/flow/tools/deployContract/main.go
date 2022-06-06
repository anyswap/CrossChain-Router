package main

import (
	"context"
	"errors"
	"flag"
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

	paramConfigFile    string
	paramChainID       string
	paramAddress       string
	paramPublicKey     string
	paramPrivKey       string
	chainID            = big.NewInt(0)
	ctx                = context.Background()
	mpcConfig          *mpc.Config
	RouterContractFile = "tokens/flow/contracts/Router.cdc"
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

	payerAddress := sdk.HexToAddress(paramAddress)
	referenceBlockID := examples.GetReferenceBlockId(flowClient)

	index, err := bridge.GetAccountIndex(paramAddress, paramPublicKey)
	if err != nil {
		log.Fatal("GetAccountIndex failed", "payerAddress", payerAddress, "err", err)
	}

	sequenceNumber, err := bridge.GetAccountNonce(paramAddress, paramPublicKey)
	if err != nil {
		log.Fatal("GetAccountNonce failed", "payerAddress", payerAddress, "err", err)
	}

	routerCode := examples.ReadFile(RouterContractFile)
	deployContractTx, err := templates.CreateAccount(nil,
		[]templates.Contract{{
			Name:   "Router",
			Source: routerCode,
		}}, payerAddress)
	if err != nil {
		log.Fatal("build deploy contract tx fails", "err", err)
	}

	deployContractTx.SetProposalKey(
		payerAddress,
		index,
		sequenceNumber,
	)
	deployContractTx.SetReferenceBlockID(referenceBlockID)
	deployContractTx.SetPayer(payerAddress)

	if paramPrivKey != "" {
		ecPrikey, err := fcrypto.DecodePrivateKeyHex(fcrypto.ECDSA_secp256k1, paramPrivKey)
		if err != nil {
			log.Fatal("DecodePrivateKeyHex failed", "privKey", paramPrivKey, "err", err)
		}

		keySigner, err := fcrypto.NewInMemorySigner(ecPrikey, fcrypto.SHA3_256)
		if err != nil {
			log.Fatal("NewInMemorySigner failed", "ecPrikey", ecPrikey, "err", err)
		}

		err = deployContractTx.SignEnvelope(deployContractTx.Payer, deployContractTx.ProposalKey.KeyIndex, keySigner)
		if err != nil {
			log.Fatal("SignEnvelope failed", "payerAddress", deployContractTx.Payer, "index", deployContractTx.ProposalKey.KeyIndex, "err", err)
		}
		txHash, err := bridge.SendTransaction(deployContractTx)
		if err != nil {
			log.Fatal("SendTransaction failed", "createAccountTx", deployContractTx, "index", index, "err", err)
		}

		log.Info("SendTransaction success", "hash", deployContractTx.ID().Hex(), "txHash", txHash)
		return
	}

	signedTx, txHash, err := MPCSignTransaction(deployContractTx, paramPublicKey)
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
	log.Warn("mpcPubkey", "mpcRealPubkey", mpcRealPubkey)
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

// =============address============= 192440c99cb17282
// =============keyIndex============= 0
// =============sig============= [137 173 22 94 93 113 255 41 214 120 81 133 137 218 4 108 136 199 98 47 120 92 23 47 145 50 94 198 140 128 184 211 150 111 146 120 230 255 81 229 102 19 138 137 213 50 77 122 134 196 36 72 218 161 166 171 244 10 138 39 83 158 165 116] 64
// ===============================sign result=============================
// KeyIndex="0" Payer="f669cb8d41ce0c74"
// len="65" sig="[6 160 181 205 10 138 41 153 221 193 217 48 103 50 62 162 182 123 145 18 31 128 239 185 234 60 176 207 68 169 213 195 82 172 8 196 207 209 27 200 116 154 120 65 253 195 113 199 115 85 234 71 158 44 61 160 240 75 5 56 27 68 25 229 1]"

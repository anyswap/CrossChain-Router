package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"flag"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/near"
	"github.com/mr-tron/base58"
	"github.com/near/borsh-go"
)

var (
	bridge = near.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramPublicKey  string
	paramTo         string
	paramAmount     string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	var err error
	var nearPubKey *near.PublicKey
	if len(paramPublicKey) == 64 {
		nearPubKey, err = near.PublicKeyFromHexString(paramPublicKey)
		if err != nil {
			log.Fatal("convert public key to address failed")
		}
	} else {
		log.Fatal("len of public key not match")
	}

	nonce, err := bridge.GetAccountNonce(nearPubKey.Address(), nearPubKey.String())
	if err != nil {
		log.Fatal("get account nonce failed", "err", err)
	}

	blockHash, err := bridge.GetLatestBlockHash()
	if err != nil {
		log.Fatal("get last block hash failed", "err", err)
	}
	blockHashBytes, err := base58.Decode(blockHash)
	if err != nil {
		log.Fatal("get last block hash failed", "err", err)
	}

	log.Info("get account nonce success", "nonce", nonce)
	actions, err := createTransferCall()
	if err != nil {
		log.Fatal("createFunctionCall failed", "err", err)
	}
	rawTx := near.CreateTransaction(nearPubKey.Address(), nearPubKey, paramTo, nonce, blockHashBytes, actions)
	signedTx, txHash, err := MPCSignTransaction(rawTx, paramPublicKey)
	if err != nil {
		log.Fatal("sign tx failed", "err", err)
	}
	log.Info("sign tx success", "txHash", txHash)

	txHash, err = bridge.SendTransaction(signedTx)
	if err != nil {
		log.Fatal("send tx failed", "err", err)
	}
	log.Info("send tx success", "txHash", txHash)
}

func createTransferCall() ([]near.Action, error) {
	log.Info("createTransferCall", "to", paramTo, "amount", paramAmount)
	if paramTo == "" || paramAmount == "" {
		return nil, errors.New("paramTo and paramAmount must input")
	}
	amount, err := common.GetBigIntFromStr(paramAmount)
	if err != nil {
		return nil, err
	}
	return []near.Action{{
		Enum: 3,
		Transfer: near.Transfer{
			Deposit: *amount,
		},
	}}, nil
}

func MPCSignTransaction(rawTx interface{}, paramPublicKey string) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*near.RawTransaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}

	buf, err := borsh.Serialize(*tx)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(buf)

	keyID, rsvs, err := mpcConfig.DoSignOneED(paramPublicKey, common.ToHex(hash[:]), "")
	if err != nil {
		return nil, "", err
	}

	if len(rsvs) != 1 {
		log.Warn("get sign status require one rsv but return many",
			"rsvs", len(rsvs), "keyID", keyID)
		return nil, "", errors.New("get sign status require one rsv but return many")
	}

	rsv := rsvs[0]
	log.Trace("get rsv signature success", "keyID", keyID, "rsv", rsv)
	sig := common.FromHex(rsv)
	if len(sig) != ed25519.SignatureSize {
		log.Error("wrong signature length", "keyID", keyID, "have", len(sig), "want", ed25519.SignatureSize)
		return nil, "", errors.New("wrong signature length")
	}

	var signature near.Signature
	signature.KeyType = 0
	copy(signature.Data[:], sig)

	var stx near.SignedTransaction
	stx.Transaction = *tx
	stx.Signature = signature

	txHash = base58.Encode(hash[:])

	log.Info("success", "keyID", keyID, "txhash", txHash)
	return &stx, txHash, nil
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramPublicKey, "pubKey", "", "signer public key")
	flag.StringVar(&paramTo, "to", "", "receive address")
	flag.StringVar(&paramAmount, "amount", "", "receive amount")

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

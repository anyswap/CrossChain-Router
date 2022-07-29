package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"flag"
	"io/ioutil"
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

	paramConfigFile    string
	paramChainID       string
	paramPublicKey     string
	paramPrivKey       string
	paramAccountId     string
	chainID            = big.NewInt(0)
	mpcConfig          *mpc.Config
	supportFuncionList = make(map[string]bool)
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	var err error
	var nearPubKey *near.PublicKey

	if paramPrivKey != "" {
		nearPubKey, err = near.PublicKeyFromString(paramPublicKey)
		if err != nil {
			log.Fatal("PublicKeyFromString", "err", err)
		}
	} else {
		if len(paramPublicKey) == 64 {
			nearPubKey, err = near.PublicKeyFromHexString(paramPublicKey)
			if err != nil {
				log.Fatal("convert public key to address failed")
			}
		} else {
			log.Fatal("len of public key not match")
		}
	}

	nonce, err := bridge.GetAccountNonce(paramAccountId, nearPubKey.String())
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
	actions, err := createFunctionCall()
	if err != nil {
		log.Fatal("createFunctionCall failed", "err", err)
	}
	rawTx := near.CreateTransaction(paramAccountId, nearPubKey, paramAccountId, nonce, blockHashBytes, actions)

	var signedTx interface{}
	var txHash string
	if paramPrivKey != "" {
		signedTx, txHash, err = bridge.SignTransactionWithPrivateKey(rawTx, paramPrivKey)
		if err != nil {
			log.Fatal("sign tx failed with paramPrivKey", "err", err, "paramPrivKey", paramPrivKey)
		}
		log.Info("sign tx success", "txHash", txHash)
	} else {
		signedTx, txHash, err = MPCSignTransaction(rawTx, paramPublicKey)
		if err != nil {
			log.Fatal("sign tx failed", "err", err)
		}
		log.Info("sign tx success", "txHash", txHash)
	}

	txHash, err = bridge.SendTransaction(signedTx)
	if err != nil {
		log.Fatal("send tx failed", "err", err)
	}
	log.Info("send tx success", "txHash", txHash)
}

//nolint:gocyclo // allow long method
func createFunctionCall() ([]near.Action, error) {
	if code, err := ioutil.ReadFile("tokens/near/tools/deployContract/mpc_pool.wasm"); err != nil {
		log.Fatal("ReadFile failed", "errf", err)
		return nil, err
	} else {
		return []near.Action{{
			Enum: 1,
			DeployContract: near.DeployContract{
				Code: code,
			},
		}}, nil
	}
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
	flag.StringVar(&paramPrivKey, "privKey", "", "signer priv key")
	flag.StringVar(&paramAccountId, "accountId", "", "signer account id")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

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

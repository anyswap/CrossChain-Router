package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/stellar"
	"github.com/btcsuite/btcd/btcec"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
)

var (
	b               *stellar.Bridge
	paramConfigFile string
	paramChainID    string
	paramPublicKey  string
	paraFee         string
	paramToAddress  string
	paramMemo       string

	paramPriKey string

	paramAmount      string
	paramAssetCode   string
	paramAssetIssuer string

	mpcConfig *mpc.Config

	chainID = big.NewInt(0)
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	var pubkeyAddr string
	var err error
	if paramPriKey != "" {
		sourceKP := keypair.MustParseFull(paramPriKey)
		pubkeyAddr = sourceKP.Address()
	} else {
		pubkeyAddr, err = stellar.PublicKeyHexToAddress(paramPublicKey)
		if err != nil {
			log.Fatal("wrong public key", "err", err)
		}
	}
	log.Infof("signer address is %v", pubkeyAddr)
	account, err := b.GetAccount(pubkeyAddr)
	if err != nil {
		log.Fatal("get account err", "err", err)
	}

	var asset txnbuild.Asset
	if paramAssetCode != "native" {
		asset = txnbuild.CreditAsset{Code: paramAssetCode, Issuer: paramAssetIssuer}
	} else {
		asset = txnbuild.NativeAsset{}
	}
	_, err = b.GetAsset(asset.GetCode(), asset.GetIssuer())
	if err != nil {
		log.Fatal("get asset err", "err", err)
	}

	memo := new(txnbuild.MemoHash)
	if paramMemo != "" {
		b, err := hex.DecodeString(paramMemo)
		if err != nil {
			log.Fatal("decode param memo error", "err", err)
		}
		copy((*memo)[:], b)
	}
	rawTx, err := buildTx(account, asset, paramToAddress, paramAmount, memo)
	if err != nil {
		log.Fatal("build tx failed", "err", err)
	}

	var signedTx interface{}
	var txHash string
	if paramPriKey != "" {
		signedTx, txHash, err = b.SignTransactionWithPrivateKey(rawTx, paramPriKey)
	} else {
		paramPublicKey, err = stellar.FormatPublicKeyToPureHex(paramPublicKey)
		if err != nil {
			log.Fatal("wrong public key", "err", err)
		}
		signedTx, txHash, err = signTx(rawTx, paramPublicKey, b)
	}
	if err != nil {
		log.Fatal("sign tx failed", "err", err)
	}
	log.Info("sign tx success", "txHash", txHash)

	txHash, err = b.SendTransaction(signedTx)
	if err != nil {
		log.Fatal("send tx failed", "err", err)
	}
	log.Info("send tx success", "txHash", txHash)
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")
	flag.StringVar(&paramPriKey, "priKey", "", "signer priKey key")
	flag.StringVar(&paramToAddress, "destination", "", "to address")
	flag.StringVar(&paraFee, "fee", "10", "(optional) fee amount")
	flag.StringVar(&paramMemo, "memo", "", "(optional) tx memo hex string")
	flag.StringVar(&paramAmount, "amount", "", "pay amount")
	flag.StringVar(&paramAssetCode, "assetCode", "", "trust asset code")
	flag.StringVar(&paramAssetIssuer, "issuer", "", "trust asset issuer")

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
	log.Info("init config finished")
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	stellar.SupportsChainID(chainID)
	b = stellar.NewCrossChainBridge(paramChainID)
	if b.NetworkStr == "" {
		log.Fatal("new bridge from chain id failed", "chainID", paramChainID)
	}
	b.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

func buildTx(
	account txnbuild.Account, asset txnbuild.Asset, toAddress, amount string, memo txnbuild.Memo) (*txnbuild.Transaction, error) {
	return txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        account,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
			Memo:                 memo,
			Operations: []txnbuild.Operation{
				&txnbuild.Payment{
					Destination: toAddress,
					Amount:      amount,
					Asset:       asset,
				},
			},
		},
	)
}

func signTx(tx *txnbuild.Transaction, pubkeyStr string, b *stellar.Bridge) (signedTx interface{}, txHash string, err error) {
	txHashBeforeSign, _ := tx.HashHex(b.NetworkStr)
	txMsg, err := network.HashTransactionInEnvelope(tx.ToXDR(), b.NetworkStr)
	if err != nil {
		return nil, "", err
	}
	var keyID string
	var rsvs []string

	signContent := common.ToHex(txMsg[:])
	keyID, rsvs, err = mpcConfig.DoSignOneED(pubkeyStr, signContent, "signTrustLineTx")

	if err != nil {
		return nil, "", err
	}
	log.Info("MPCSignTransaction finished", "keyID", keyID)

	if len(rsvs) != 1 {
		return nil, "", fmt.Errorf("get sign status require one rsv but have %v (keyID = %v)", len(rsvs), keyID)
	}

	rsv := rsvs[0]
	log.Trace("MPCSignTransaction get rsv success", "keyID", keyID, "rsv", rsv)

	sig := rsvToSig(rsv, true)

	pubkeyAddr, _ := b.PublicKeyToAddress(pubkeyStr)
	pubkeyKeyPair := keypair.MustParseAddress(pubkeyAddr)

	err = pubkeyKeyPair.Verify(txMsg[:], sig)
	if err != nil {
		return nil, "", fmt.Errorf("stellar verify signature error : %v", err)
	}
	newSignedTx, err := stellar.MakeSignedTransaction(pubkeyKeyPair, sig, tx)
	if err != nil {
		return signedTx, "", err
	}

	txhash, err := newSignedTx.HashHex(b.NetworkStr)
	if err != nil {
		return signedTx, "", err
	}

	if txHashBeforeSign != txhash {
		return nil, "", fmt.Errorf("stellar verify signature error : %v %v", txHashBeforeSign, txhash)
	}

	return newSignedTx, txhash, nil
}

func rsvToSig(rsv string, isEd bool) []byte {
	if isEd {
		return common.FromHex(rsv)
	}
	b, _ := hex.DecodeString(rsv)
	rx := hex.EncodeToString(b[:32])
	sx := hex.EncodeToString(b[32:64])
	r, _ := new(big.Int).SetString(rx, 16)
	s, _ := new(big.Int).SetString(sx, 16)
	signature := &btcec.Signature{
		R: r,
		S: s,
	}
	return signature.Serialize()
}

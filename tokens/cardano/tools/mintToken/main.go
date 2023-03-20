package main

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
	"github.com/cosmos/btcutil/bech32"
	cardanosdk "github.com/echovl/cardano-go"
	"github.com/echovl/cardano-go/crypto"
)

var (
	b = cardano.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramFrom       string
	paramTo         string
	paramAsset      string
	paramAmount     string
	paramPubkey     string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	_, _, policyID := b.GetAssetPolicy(paramAsset)
	assetName := cardanosdk.NewAssetName(paramAsset)

	paramAmount, err := common.GetBigIntFromStr(paramAmount)
	if err != nil {
		panic(err)
	}

	assetNameWithPolicy := policyID.String() + "." + common.Bytes2Hex(assetName.Bytes())
	fmt.Printf("mint asset: %s amount: %d to: %s", assetNameWithPolicy, paramAmount.Int64(), paramTo)

	utxos, err := b.QueryUtxo(paramFrom, assetNameWithPolicy, paramAmount)
	if err != nil {
		panic(err)
	}

	swapId := fmt.Sprintf("mint_%s_%d", paramAsset, time.Now().Unix())
	rawTx, err := b.BuildTx(swapId, paramTo, assetNameWithPolicy, paramAmount, utxos)
	if err != nil {
		panic(err)
	}
	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			SwapID: swapId,
		},
		From:  paramFrom,
		Extra: &tokens.AllExtras{},
	}

	tx, err := b.CreateRawTx(rawTx, b.GetRouterContract(""))
	if err != nil {
		panic(err)
	}

	mpcParams := params.GetMPCConfig(b.UseFastMPC)
	var signTx *cardano.SignedTransaction

	if mpcParams.SignWithPrivateKey {
		priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
		signTx, _, _ = b.SignTransactionWithPrivateKey(tx, rawTx, args, priKey)
	} else {
		signingMsg, err := tx.Hash()
		if err != nil {
			panic(err)
		}

		jsondata, _ := json.Marshal(args.GetExtraArgs())
		msgContext := string(jsondata)

		txid := args.SwapID
		logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
		log.Info(logPrefix+"start", "txid", txid, "signingMsg", signingMsg.String())

		keyID, rsvs, err := mpcConfig.DoSignOneED(paramPubkey, signingMsg.String(), msgContext)
		if err != nil {
			panic(err)
		}

		if len(rsvs) != 1 {
			log.Warn("get sign status require one rsv but return many",
				"rsvs", len(rsvs), "keyID", keyID, "txid", txid)
			panic(errors.New("get sign status require one rsv but return many"))
		}

		rsv := rsvs[0]
		log.Trace(logPrefix+"get rsv signature success", "keyID", keyID, "txid", txid, "rsv", rsv)
		sig := common.FromHex(rsv)
		if len(sig) != ed25519.SignatureSize {
			log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(sig), "want", ed25519.SignatureSize)
			panic(errors.New("wrong signature length"))
		}

		pubStr, _ := bech32.EncodeFromBase256("addr_vk", common.FromHex(paramPubkey))
		pubKey, _ := crypto.NewPubKey(pubStr)
		b.AppendSignature(tx, pubKey, sig)

		cacheAssetsMap := rawTx.TxOuts[args.From]
		txInputs := rawTx.TxIns
		txIndex := rawTx.TxIndex
		signTx = &cardano.SignedTransaction{
			TxIns:     txInputs,
			TxHash:    signingMsg.String(),
			TxIndex:   txIndex,
			AssetsMap: cacheAssetsMap,
			Tx:        tx,
		}
	}

	if txHash, err := b.SendTransaction(signTx); err != nil {
		panic(err)
	} else {
		fmt.Printf("txHash: %s", txHash)
	}

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
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	b.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})

	b.SetChainConfig(&tokens.ChainConfig{
		BlockChain:     "Cardano",
		ChainID:        chainID.String(),
		RouterContract: paramFrom,
		Confirmations:  1,
	})

	_ = b.GetChainConfig().CheckConfig()

	b.InitAfterConfig()

	log.Info("init bridge finished")

}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramFrom, "from", "", "mpc address")
	flag.StringVar(&paramTo, "to", "", "receive address")
	flag.StringVar(&paramAmount, "amount", "", "receive amount")
	flag.StringVar(&paramAsset, "asset", "", "asset eg. USDT")
	flag.StringVar(&paramPubkey, "pubkey", "", "pubkey")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}
	if paramPubkey != "" {
		router.SetMPCPublicKey(paramFrom, paramPubkey)
	}

	log.Info("init flags finished")
}

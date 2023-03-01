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
	bind            string
	toChainId       string
	paramPubkey     string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	// policy := strings.Split(paramAsset, ".")
	// if len(policy) != 2 {
	// 	panic("policy format error")
	// }

	// _, _, policyID := b.GetAssetPolicy(paramAsset)
	// assetName := cardanosdk.NewAssetName(paramAsset)
	// assetNameWithPolicy := policyID.String() + "." + common.Bytes2Hex(assetName.Bytes())

	paramAmount, err := common.GetBigIntFromStr(paramAmount)
	if err != nil {
		panic(err)
	}

	fmt.Printf("send asset: %s amount: %d to: %s", paramAsset, paramAmount.Int64(), paramTo)

	utxos, err := b.QueryUtxo(paramFrom, paramAsset, paramAmount)
	if err != nil {
		panic(err)
	}

	swapId := fmt.Sprintf("send_%s_%d", paramAsset, time.Now().Unix())
	rawTx, err := b.BuildTx(swapId, paramTo, paramAsset, paramAmount, utxos)
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

	tx, err := b.CreateSwapoutRawTx(rawTx, b.GetRouterContract(""), bind, toChainId)
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
	log.Info("init bridge finished")

	b.SetChainConfig(&tokens.ChainConfig{
		BlockChain:     "Cardano",
		ChainID:        chainID.String(),
		RouterContract: paramFrom,
		Confirmations:  1,
	})

	_ = b.GetChainConfig().CheckConfig()

	b.InitAfterConfig()

	if paramPubkey != "" {
		router.SetMPCPublicKey(paramFrom, paramPubkey)
	}

}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramFrom, "from", "", "mpc address")
	flag.StringVar(&paramTo, "to", "", "receive address")
	flag.StringVar(&paramAmount, "amount", "", "receive amount")
	flag.StringVar(&paramAsset, "asset", "", "asset With Policy e.g.f0573f98953b187eec04b21eb25a5983d9d03b0d87223c768555b2ec.55534454")
	flag.StringVar(&bind, "bind", "", "to address")
	flag.StringVar(&toChainId, "toChainId", "", "toChainId")
	flag.StringVar(&paramPubkey, "pubkey", "", "mpc pubkey")

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

package main

import (
	"crypto/ed25519"
	"flag"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
)

var (
	bridge = cardano.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramFrom       string
	paramPublicKey  string
	paramTo         string
	paramAsset      string
	paramAmount     string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	if len(paramPublicKey) != 64 {
		log.Fatal("len of public key not match")
	}

	if value, err := common.GetBigIntFromStr(paramAmount); err != nil {
		log.Fatal("GetBigIntFromStr fails", "paramAmount", paramAmount)
	} else {
		if utxos, err := bridge.QueryUtxoOnChain(paramFrom); err != nil {
			log.Fatal("QueryUtxo", "err", err)
		} else {
			if rawTransaction, err := bridge.BuildTx("swapId", paramTo, paramAsset, value, utxos); err != nil {
				if err := cardano.CreateRawTx(rawTransaction, paramFrom); err != nil {
					log.Fatal("CreateRawTx", "err", err)
				} else {
					if minFee, err := cardano.CalcMinFee(rawTransaction); err != nil {
						log.Fatal("CalcMinFee", "err", err)
					} else {
						if feeList := strings.Split(minFee, " "); len(feeList) != 2 {
							log.Fatal("feeList length not match")
						} else {
							rawTransaction.Fee = feeList[0]
							if adaAmount, err := common.GetBigIntFromStr(rawTransaction.TxOuts[paramFrom][paramAsset]); err != nil {
								log.Fatal("GetBigIntFromStr", "err", err)
							} else {
								if feeAmount, err := common.GetBigIntFromStr(feeList[0]); err != nil {
									log.Fatal("GetBigIntFromStr", "err", err)
								} else {
									returnAmount := adaAmount.Sub(adaAmount, feeAmount)
									if returnAmount.Cmp(cardano.FixAdaAmount) < 0 {
										log.Fatal("return value less than min value")
									} else {
										rawTransaction.TxOuts[paramFrom][paramAsset] = returnAmount.String()
										if err := cardano.CreateRawTx(rawTransaction, paramFrom); err != nil {
											log.Fatal("CreateRawTx", "err", err)
										} else {
											txPath := cardano.RawPath + rawTransaction.OutFile + cardano.RawSuffix
											witnessPath := cardano.WitnessPath + rawTransaction.OutFile + cardano.WitnessSuffix
											signedPath := cardano.SignedPath + rawTransaction.OutFile + cardano.SignedSuffix
											if txHash, err := cardano.CalcTxId(txPath); err != nil {
												log.Fatal("CalcTxId", "err", err)
											} else {
												if keyID, rsvs, err := mpcConfig.DoSignOneED(paramPublicKey, txHash, ""); err != nil {
													log.Fatal("DoSignOneED", "err", err)
												} else {
													if len(rsvs) != 1 {
														log.Fatal("get sign status require one rsv but return many",
															"rsvs", len(rsvs), "keyID", keyID)
													}

													rsv := rsvs[0]
													sig := common.FromHex(rsv)
													if len(sig) != ed25519.SignatureSize {
														log.Fatal("wrong signature length", "keyID", keyID, "have", len(sig), "want", ed25519.SignatureSize)
													}

													if err := bridge.CreateWitness(witnessPath, paramPublicKey, sig); err != nil {
														log.Fatal("CreateWitness", "err", err)
													} else {
														if err := bridge.SignTx(txPath, witnessPath, signedPath); err != nil {
															log.Fatal("SignTx", "err", err)
														} else {
															if txHash, err := bridge.SendTransaction(&cardano.SignedTransaction{
																FilePath: signedPath,
																TxHash:   txHash,
															}); err != nil {
																log.Fatal("SendTransaction", "err", err)
															} else {
																log.Info("SendTransaction", "txHash", txHash)
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

	}

}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	mpcConfig = mpc.InitConfig(config.FastMPC, true)
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

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramPublicKey, "pubKey", "", "signer public key")
	flag.StringVar(&paramFrom, "from", "", "sender address")
	flag.StringVar(&paramTo, "to", "", "receive address")
	flag.StringVar(&paramAmount, "amount", "", "receive amount")
	flag.StringVar(&paramAsset, "asset", "", "asset")

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

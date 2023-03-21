package main

import (
	"errors"
	"flag"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	paramURLs       string
	paramConfigFile string
	paramChainID    string
	paramPrefix     string
	paramSender     string
	paramTo         string
	paramDenom      string
	paramAmount     uint64
	paramMemo       string
	paramGasLimit   = uint64(200000)
	paramFee        string
	paramSequence   uint64
	paramPublicKey  string
	paramPrivateKey string
	paramUseGrpc    bool

	chainID = big.NewInt(0)
	bridge  = cosmos.NewCrossChainBridge()

	mpcConfig *mpc.Config
)

func main() {
	initAll()
	if rawTx, err := BuildTx(); err != nil {
		log.Fatalf("BuildTx err:%+v", err)
	} else {
		var signedTx interface{}
		var txHash string
		if paramPrivateKey != "" {
			if signedTx, txHash, err = bridge.SignTransactionWithPrivateKey(rawTx, paramPrivateKey); err != nil {
				log.Fatalf("SignTransactionWithPrivateKey err:%+v", err)
			}
		} else {
			if signedTx, txHash, err = MPCSignTransaction(rawTx, paramPublicKey); err != nil {
				log.Fatalf("MPCSignTransaction err:%+v", err)
			}
		}
		if txHashFromSend, err := bridge.SendTransaction(signedTx); err != nil {
			log.Fatalf("SendTransaction err:%+v", err)
		} else {
			log.Printf("txhash: %+s txHashFromSend: %+s", txHash, txHashFromSend)
		}
	}
}

func initExtra() (*tokens.AllExtras, error) {
	extra := &tokens.AllExtras{}
	if account, err := bridge.GetBaseAccount(paramSender); err != nil {
		return nil, err
	} else {
		if extra.Sequence == nil {
			if paramSequence > 0 {
				extra.Sequence = &paramSequence
			} else if sequence, err := strconv.ParseUint(account.Account.Sequence, 10, 64); err == nil {
				extra.Sequence = &sequence
			} else {
				return nil, err
			}
		}

		if extra.Gas == nil {
			extra.Gas = &paramGasLimit
		}
		if extra.Fee == nil {
			extra.Fee = &paramFee
		}

		return extra, nil
	}
}

func BuildTx() (*cosmos.BuildRawTx, error) {
	if extra, err := initExtra(); err != nil {
		return nil, err
	} else {
		txBuilder := bridge.TxConfig.NewTxBuilder()
		amount := sdk.NewIntFromUint64(paramAmount)
		sendMsg := cosmos.BuildSendMsg(paramSender, paramTo, paramDenom, amount.BigInt())
		if err := txBuilder.SetMsgs(sendMsg); err != nil {
			log.Fatalf("SetMsgs error:%+v", err)
		}
		txBuilder.SetMemo(paramMemo)
		if fee, err := cosmos.ParseCoinsFee(*extra.Fee); err != nil {
			log.Fatalf("ParseCoinsFee error:%+v", err)
		} else {
			txBuilder.SetFeeAmount(fee)
		}
		txBuilder.SetGasLimit(*extra.Gas)
		pubKey, err := cosmos.PubKeyFromStr(paramPublicKey)
		if err != nil {
			log.Fatalf("PubKeyFromStr error:%+v", err)
		}
		sig := cosmos.BuildSignatures(pubKey, *extra.Sequence, nil)
		if err := txBuilder.SetSignatures(sig); err != nil {
			log.Fatalf("SetSignatures error:%+v", err)
		}
		if err := txBuilder.GetTx().ValidateBasic(); err != nil {
			log.Fatalf("ValidateBasic error:%+v", err)
		}
		accountNumber, err := bridge.GetAccountNum(paramSender)
		if err != nil {
			return nil, err
		}
		return &cosmos.BuildRawTx{
			TxBuilder:     txBuilder,
			AccountNumber: accountNumber,
			Sequence:      *extra.Sequence,
		}, nil
	}
}

func MPCSignTransaction(tx *cosmos.BuildRawTx, publicKey string) (signedTx interface{}, txHash string, err error) {
	mpcPubkey := publicKey
	pubKey, err := cosmos.PubKeyFromStr(mpcPubkey)
	if err != nil {
		return nil, txHash, err
	}
	if signBytes, err := bridge.GetSignBytes(tx); err != nil {
		return nil, "", err
	} else {
		msgHash := fmt.Sprintf("%X", cosmos.Sha256Sum(signBytes))
		if keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, msgHash, ""); err != nil {
			return nil, "", err
		} else {
			if len(rsvs) != 1 {
				log.Warn("get sign status require one rsv but return many",
					"rsvs", len(rsvs), "keyID", keyID)
				return nil, "", errors.New("get sign status require one rsv but return many")
			}

			rsv := rsvs[0]
			signature := common.FromHex(rsv)

			if len(signature) == crypto.SignatureLength {
				signature = signature[:crypto.SignatureLength-1]
			}

			if len(signature) != crypto.SignatureLength-1 {
				log.Error("wrong signature length", "keyID", keyID, "have", len(signature), "want", crypto.SignatureLength)
				return nil, "", errors.New("wrong signature length")
			}

			if !pubKey.VerifySignature(signBytes, signature) {
				log.Error("verify signature failed", "signBytes", common.ToHex(signBytes), "signature", signature)
				return nil, "", errors.New("wrong signature")
			}

			sequence := tx.Sequence
			sig := cosmos.BuildSignatures(pubKey, sequence, signature)
			txBuilder := tx.TxBuilder
			if err := txBuilder.SetSignatures(sig); err != nil {
				return nil, "", err
			}

			return bridge.GetSignTx(txBuilder.GetTx())
		}
	}
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(),
			`Usage:
this command support two ways to sign tx
1. use private key
	use paramters: privateKey, url, grpc
2. use mpc
	use parameters: config

Parameters:`)
		flag.PrintDefaults()
	}

	flag.StringVar(&paramURLs, "url", "", "urls (comma separated)")
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramPrefix, "prefix", "", "bech32 prefix for account")
	flag.StringVar(&paramSender, "sender", "", "sender address")
	flag.StringVar(&paramTo, "to", "", "to address")
	flag.StringVar(&paramDenom, "denom", "", "denom")
	flag.Uint64Var(&paramAmount, "amount", paramAmount, "mint amount")
	flag.Uint64Var(&paramGasLimit, "gasLimit", paramGasLimit, "gas limit")
	flag.Uint64Var(&paramSequence, "sequence", paramSequence, "sequence number")
	flag.StringVar(&paramFee, "fee", "", "tx fee")
	flag.StringVar(&paramPublicKey, "publicKey", "", "public Key")
	flag.StringVar(&paramPrivateKey, "privateKey", "", "private key")
	flag.StringVar(&paramMemo, "memo", "", "tx memo")
	flag.BoolVar(&paramUseGrpc, "grpc", paramUseGrpc, "use grpc call")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

	if paramPrefix == "" {
		log.Fatal("must cofig -prefix")
	}
	if paramDenom == "" {
		log.Fatal("must cofig -denom")
	}
	if paramFee == "" {
		log.Fatal("must cofig -fee")
	}
	if paramSender == "" || paramTo == "" {
		log.Fatal("must cofig -sender and -to")
	}
	if paramPublicKey == "" {
		log.Fatal("must cofig -publicKey")
	}
	if paramPrivateKey == "" || paramConfigFile == "" {
		log.Fatal("must cofig -privateKey or -config")
	}

	log.Info("init flags finished")
}

func initConfig() {
	if paramConfigFile == "" {
		return
	}

	config := params.LoadRouterConfig(paramConfigFile, true, false)
	if config.FastMPC != nil {
		mpcConfig = mpc.InitConfig(config.FastMPC, true)
	} else {
		mpcConfig = mpc.InitConfig(config.MPC, true)
	}
	log.Info("init config finished", "IsFastMPC", mpcConfig.IsFastMPC)
}

func initBridge() {
	if paramConfigFile != "" {
		cfg := params.GetRouterConfig()
		apiAddrs := cfg.Gateways[chainID.String()]
		apiAddrsExt := cfg.GatewaysExt[chainID.String()]
		grpcAPIs := cfg.GRPCGateways[chainID.String()]
		bridge.SetGatewayConfig(&tokens.GatewayConfig{
			APIAddress:     apiAddrs,
			APIAddressExt:  apiAddrsExt,
			GRPCAPIAddress: grpcAPIs,
		})
		log.Info("use config file", "config", paramConfigFile)
	} else {
		gateway := &tokens.GatewayConfig{}
		if paramUseGrpc {
			gateway.GRPCAPIAddress = strings.Split(paramURLs, ",")
		} else {
			gateway.APIAddress = strings.Split(paramURLs, ",")
		}
		bridge.SetGatewayConfig(gateway)
		log.Info("use direct urls", "paramURLs", paramURLs, "paramUseGrpc", paramUseGrpc)
	}
	log.Infof("gateway config is %v", common.ToJSONString(bridge.GetGatewayConfig(), false))
	bridge.SetChainConfig(&tokens.ChainConfig{
		ChainID: chainID.String(),
	})

	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(paramPrefix, "")
	config.Seal()
}

package main

import (
	"flag"
	"math/big"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	paramURLs       string
	paramChainID    string
	paramPrefix     string
	paramSender     string
	paramTo         string
	paramDenom      string
	paramAmount     uint64
	paramPublicKey  string
	paramPrivateKey string
	paramMemo       string
	paramGasLimit   = uint64(200000)
	paramFee        = "1usei"
	paramSequence   uint64

	chainID = big.NewInt(0)
	bridge  = cosmos.NewCrossChainBridge()
)

func main() {
	initFlags()
	initBridge()

	if rawTx, err := BuildTx(); err != nil {
		log.Fatalf("BuildTx err:%+v", err)
	} else {
		if signedTx, txHash, err := bridge.SignTransactionWithPrivateKey(rawTx, paramPrivateKey); err != nil {
			log.Fatalf("SignTransactionWithPrivateKey err:%+v", err)
		} else {
			if txHashFromSend, err := bridge.SendTransaction(signedTx); err != nil {
				log.Fatalf("SendTransaction err:%+v", err)
			} else {
				log.Printf("txhash: %+s txHashFromSend: %+s", txHash, txHashFromSend)
			}
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

func initFlags() {
	flag.StringVar(&paramURLs, "url", "https://sei-testnet-rpc.allthatnode.com:1317", "urls (comma separated)")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramPrefix, "prefix", "sei", "bech32 prefix for account")
	flag.StringVar(&paramSender, "sender", "", "sender address")
	flag.StringVar(&paramTo, "to", "", "to address")
	flag.StringVar(&paramDenom, "denom", "", "denom")
	flag.Uint64Var(&paramAmount, "amount", paramAmount, "mint amount")
	flag.Uint64Var(&paramGasLimit, "gasLimit", paramGasLimit, "gas limit")
	flag.Uint64Var(&paramSequence, "sequence", paramSequence, "sequence number")
	flag.StringVar(&paramFee, "fee", paramFee, "tx fee")
	flag.StringVar(&paramPublicKey, "publicKey", "", "public Key")
	flag.StringVar(&paramPrivateKey, "privateKey", "", "private key")
	flag.StringVar(&paramMemo, "memo", "", "tx memo")

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

func initBridge() {
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress: strings.Split(paramURLs, ","),
	})
	bridge.SetChainConfig(&tokens.ChainConfig{
		ChainID: chainID.String(),
	})

	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(paramPrefix, "")
	config.Seal()
}

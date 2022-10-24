package main

import (
	"errors"
	"flag"
	"fmt"
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosRouter"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	tokenfactoryTypes "github.com/sei-protocol/sei-chain/x/tokenfactory/types"
)

var (
	bridge          = cosmosRouter.NewCrossChainBridge()
	paramConfigFile string
	paramChainID    string
	paramSender     string
	paramSubdenom   string
	paramMemo       string
	paramFee        string
	paramPublicKey  string
	paramPrivateKey string
	chainID         = big.NewInt(0)
	mpcConfig       *mpc.Config

	DefaultGasLimit uint64 = 200000
)

func main() {
	log.SetLogger(6, false, true)

	initAll()
	urls := bridge.GetGatewayConfig().APIAddress
	client := cosmosSDK.NewCosmosRestClient(urls, "", "")
	if rawTx, err := BuildTx(client); err != nil {
		log.Fatalf("BuildTx err:%+v", err)
	} else {
		var signedTx interface{}
		var txHash string
		if paramPrivateKey != "" {
			if signedTx, txHash, err = SignTransactionWithPrivateKey(client, *rawTx.TxBuilder, paramPrivateKey, rawTx.Extra); err != nil {
				log.Fatalf("SignTransactionWithPrivateKey err:%+v", err)
			}
		} else {
			if signedTx, txHash, err = MPCSignTransaction(client, *rawTx.TxBuilder, paramPublicKey, rawTx.Extra); err != nil {
				log.Fatalf("SignTransactionWithPrivateKey err:%+v", err)
			}
		}
		if txHashFromSend, err := client.SendTransaction(signedTx); err != nil {
			log.Fatalf("SendTransaction err:%+v", err)
		} else {
			fmt.Printf("txhash: %+s txHashFromSend: %+s", txHash, txHashFromSend)
		}
	}
}

func BuildCreateDenomMsg() *tokenfactoryTypes.MsgCreateDenom {
	return tokenfactoryTypes.NewMsgCreateDenom(paramSender, paramSubdenom)
}

func initExtra(client *cosmosSDK.CosmosRestClient) (*tokens.AllExtras, error) {
	extra := &tokens.AllExtras{}
	if account, err := client.GetBaseAccount(paramSender); err != nil {
		return nil, err
	} else {
		if extra.Sequence == nil {
			if sequence, err := strconv.ParseUint(account.Account.Sequence, 10, 64); err == nil {
				extra.Sequence = &sequence
			} else {
				return nil, err
			}
		}

		if extra.AccountNum == nil {
			if accountNumber, err := strconv.ParseUint(account.Account.AccountNumber, 10, 64); err == nil {
				extra.AccountNum = &accountNumber
			} else {
				return nil, err
			}
		}

		if extra.Gas == nil {
			extra.Gas = &DefaultGasLimit
		}
		if extra.Fee == nil {
			extra.Fee = &paramFee
		}

		return extra, nil
	}
}

func BuildTx(client *cosmosSDK.CosmosRestClient) (*cosmosSDK.BuildRawTx, error) {
	if extra, err := initExtra(client); err != nil {
		return nil, err
	} else {
		txBuilder := client.TxConfig.NewTxBuilder()
		msg := BuildCreateDenomMsg()
		if err := txBuilder.SetMsgs(msg); err != nil {
			log.Fatalf("SetMsgs error:%+v", err)
		}
		txBuilder.SetMemo(paramMemo)
		if fee, err := cosmosSDK.ParseCoinsFee(*extra.Fee); err != nil {
			log.Fatalf("ParseCoinsFee error:%+v", err)
		} else {
			txBuilder.SetFeeAmount(fee)
		}
		txBuilder.SetGasLimit(DefaultGasLimit)
		pubKey, err := cosmosSDK.PubKeyFromStr(paramPublicKey)
		if err != nil {
			log.Fatalf("PubKeyFromStr error:%+v", err)
		}
		sig := cosmosSDK.BuildSignatures(pubKey, *extra.Sequence, nil)
		if err := txBuilder.SetSignatures(sig); err != nil {
			log.Fatalf("SetSignatures error:%+v", err)
		}
		if err := txBuilder.GetTx().ValidateBasic(); err != nil {
			log.Fatalf("ValidateBasic error:%+v", err)
		}
		return &cosmosSDK.BuildRawTx{
			TxBuilder: &txBuilder,
			Extra:     extra,
		}, nil
	}
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func SignTransactionWithPrivateKey(client *cosmosSDK.CosmosRestClient, txBuilder cosmosClient.TxBuilder, privKey string, extras *tokens.AllExtras) (signedTx interface{}, txHash string, err error) {
	if ecPrikey, err := crypto.HexToECDSA(privKey); err != nil {
		return nil, "", err
	} else {
		ecPriv := &secp256k1.PrivKey{Key: ecPrikey.D.Bytes()}

		if signBytes, err := client.GetSignBytes(txBuilder, *extras.AccountNum, *extras.Sequence); err != nil {
			return nil, "", err
		} else {
			if signature, err := ecPriv.Sign(signBytes); err != nil {
				return nil, "", err
			} else {
				if len(signature) == crypto.SignatureLength {
					signature = signature[:crypto.SignatureLength-1]
				}

				if len(signature) != crypto.SignatureLength-1 {
					log.Fatal("wrong length of signature", "length", len(signature))
					return nil, "", errors.New("wrong signature length")
				}

				pubKey := ecPriv.PubKey()
				if !pubKey.VerifySignature(signBytes, signature) {
					log.Fatal("verify signature failed", "signBytes", common.ToHex(signBytes), "signature", signature)
					return nil, "", errors.New("wrong signature")
				}
				sig := cosmosSDK.BuildSignatures(pubKey, *extras.Sequence, signature)
				if err := txBuilder.SetSignatures(sig); err != nil {
					return nil, "", err
				}
				if err := txBuilder.GetTx().ValidateBasic(); err != nil {
					return nil, "", err
				}

				return client.GetSignTx(txBuilder.GetTx())
			}
		}
	}
}

func MPCSignTransaction(client *cosmosSDK.CosmosRestClient, txBuilder cosmosClient.TxBuilder, publicKey string, extras *tokens.AllExtras) (signedTx interface{}, txHash string, err error) {
	mpcPubkey := publicKey
	pubKey, err := cosmosSDK.PubKeyFromStr(mpcPubkey)
	if err != nil {
		return nil, txHash, err
	}
	if signBytes, err := client.GetSignBytes(txBuilder, *extras.AccountNum, *extras.Sequence); err != nil {
		return nil, "", err
	} else {

		msgHash := fmt.Sprintf("%X", cosmosSDK.Sha256Sum(signBytes))
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
			sig := cosmosSDK.BuildSignatures(pubKey, *extras.Sequence, signature)
			if err := txBuilder.SetSignatures(sig); err != nil {
				return nil, "", err
			}

			return client.GetSignTx(txBuilder.GetTx())
		}
	}
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {

	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramSender, "sender", "", "token creater")
	flag.StringVar(&paramSubdenom, "denom", "", "token denom")
	flag.StringVar(&paramMemo, "memo", "", "transaction memo")
	flag.StringVar(&paramFee, "fee", "", "transaction fee")
	flag.StringVar(&paramPublicKey, "publicKey", "", "public Key")
	flag.StringVar(&paramPrivateKey, "privateKey", "", "(optional) private key")

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

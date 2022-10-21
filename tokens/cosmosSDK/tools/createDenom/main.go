package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	// tokenfactoryTypes "github.com/sei-protocol/sei-chain/x/tokenfactory/types"
)

var (
	Sender                 = "sei1hak8tpyulsw3up5ltgtx9t5usupysjjkmsqgn3"
	Subdenom               = "uany1"
	Memo                   = "test BuildCreateDenomMsg"
	Fee                    = "1usei"
	DefaultGasLimit uint64 = 100000
	publicKey              = "0x04eec4c8fe47be3f4f2576bff7a1c45a48363f2ae65afa03e32531bcecfaa02b2085118b678884c9862ff713767e2fd4bd99b8fe1ff23042ffd7ad9260193d13fc"
	privateKey             = "861626f2765b464e59cbd7c1a53e2145232676b6e4139a73f9edf829f0470b74"
	url                    = []string{"https://sei-testnet-rpc.allthatnode.com:1317"}
)

func main() {
	client := cosmosSDK.NewCosmosRestClient(url)
	if rawTx, err := BuildTx(client); err != nil {
		log.Fatalf("BuildTx err:%+v", err)
	} else {
		if signedTx, txHash, err := SignTransactionWithPrivateKey(client, *rawTx.TxBuilder, privateKey, rawTx.Extra); err != nil {
			log.Fatalf("SignTransactionWithPrivateKey err:%+v", err)
		} else {
			if txHashFromSend, err := client.SendTransaction(signedTx); err != nil {
				log.Fatalf("SendTransaction err:%+v", err)
			} else {
				fmt.Printf("txhash:%+s txHashFromSend:%+s", txHash, txHashFromSend)
			}
		}
	}
}

func BuildCreateDenomMsg() *cosmosSDK.MsgCreateDenom {
	return &cosmosSDK.MsgCreateDenom{
		Sender:   Sender,
		Subdenom: Subdenom,
	}
}

// func BuildCreateDenomMsg() *tokenfactoryTypes.MsgCreateDenom {
// 	return tokenfactoryTypes.NewMsgCreateDenom(Sender, Subdenom)
// }

func initExtra(client *cosmosSDK.CosmosRestClient) (*tokens.AllExtras, error) {
	extra := &tokens.AllExtras{}
	if account, err := client.GetBaseAccount(Sender); err != nil {
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
			extra.Fee = &Fee
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
		txBuilder.SetMemo(Memo)
		if fee, err := cosmosSDK.ParseCoinsFee(*extra.Fee); err != nil {
			log.Fatalf("ParseCoinsFee error:%+v", err)
		} else {
			txBuilder.SetFeeAmount(fee)
		}
		txBuilder.SetGasLimit(DefaultGasLimit)
		pubKey, err := cosmosSDK.PubKeyFromStr(publicKey)
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
		pubKey := ecPriv.PubKey()

		if signBytes, err := client.GetSignBytes(txBuilder, Sender, *extras.AccountNum, *extras.Sequence, pubKey); err != nil {
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

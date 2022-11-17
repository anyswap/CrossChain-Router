package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmos"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types"
)

var (
	Sender                 = ""
	Amount                 = types.NewCoin("", types.NewIntFromUint64(100000))
	Memo                   = "test mintToken"
	Fee                    = "1usei"
	DefaultGasLimit uint64 = 200000
	publicKey              = ""
	privateKey             = ""
	urls                   = []string{"https://sei-testnet-rpc.allthatnode.com:1317"}

	bridge = cosmos.NewCrossChainBridge()
)

func main() {
	bridge.SetGatewayConfig(&tokens.GatewayConfig{APIAddress: urls})

	if rawTx, err := BuildTx(); err != nil {
		log.Fatalf("BuildTx err:%+v", err)
	} else {
		if signedTx, txHash, err := SignTransactionWithPrivateKey(*rawTx.TxBuilder, privateKey, rawTx.Extra); err != nil {
			log.Fatalf("SignTransactionWithPrivateKey err:%+v", err)
		} else {
			if txHashFromSend, err := bridge.SendTransaction(signedTx); err != nil {
				log.Fatalf("SendTransaction err:%+v", err)
			} else {
				fmt.Printf("txhash: %+s txHashFromSend: %+s", txHash, txHashFromSend)
			}
		}
	}
}

func initExtra() (*tokens.AllExtras, error) {
	extra := &tokens.AllExtras{}
	if account, err := bridge.GetBaseAccount(Sender); err != nil {
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

func BuildTx() (*cosmos.BuildRawTx, error) {
	if extra, err := initExtra(); err != nil {
		return nil, err
	} else {
		txBuilder := bridge.TxConfig.NewTxBuilder()
		mintMsg := cosmos.BuildMintMsg(Sender, Amount)
		if err := txBuilder.SetMsgs(mintMsg); err != nil {
			log.Fatalf("SetMsgs error:%+v", err)
		}

		txBuilder.SetMemo(Memo)
		if fee, err := cosmos.ParseCoinsFee(*extra.Fee); err != nil {
			log.Fatalf("ParseCoinsFee error:%+v", err)
		} else {
			txBuilder.SetFeeAmount(fee)
		}
		txBuilder.SetGasLimit(DefaultGasLimit)
		pubKey, err := cosmos.PubKeyFromStr(publicKey)
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
		return &cosmos.BuildRawTx{
			TxBuilder: &txBuilder,
			Extra:     extra,
		}, nil
	}
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func SignTransactionWithPrivateKey(txBuilder cosmosClient.TxBuilder, privKey string, extras *tokens.AllExtras) (signedTx interface{}, txHash string, err error) {
	if ecPrikey, err := crypto.HexToECDSA(privKey); err != nil {
		return nil, "", err
	} else {
		ecPriv := &secp256k1.PrivKey{Key: ecPrikey.D.Bytes()}

		if signBytes, err := bridge.GetSignBytes(txBuilder, *extras.AccountNum, *extras.Sequence); err != nil {
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
				sig := cosmos.BuildSignatures(pubKey, *extras.Sequence, signature)
				if err := txBuilder.SetSignatures(sig); err != nil {
					return nil, "", err
				}
				if err := txBuilder.GetTx().ValidateBasic(); err != nil {
					return nil, "", err
				}

				return bridge.GetSignTx(txBuilder.GetTx())
			}
		}
	}
}

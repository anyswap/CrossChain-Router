package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosHub"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codecTypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var (
	paramPublicKey string
	paramPrivKey   string
	paramFrom      string
	paramTo        string
	paramValue     string
	paramDenom     string
	paramMemo      string
	url            = "https://cosmos-mainnet-rpc.allthatnode.com:1317"
)

func initFlags() {
	flag.StringVar(&paramPublicKey, "public", "", "publicKey")
	flag.StringVar(&paramPrivKey, "priv", "", "privKey")
	flag.StringVar(&paramFrom, "from", "", "from")
	flag.StringVar(&paramTo, "to", "", "to")
	flag.StringVar(&paramValue, "value", "", "value")
	flag.StringVar(&paramDenom, "denom", "", "denom")
	flag.StringVar(&paramMemo, "memo", "", "memo")

	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)

	initFlags()

	if !cosmosSDK.IsValidAddress(paramFrom) {
		log.Fatal("from address valid faild", "address", paramFrom)
	}

	if !cosmosSDK.IsValidAddress(paramTo) {
		log.Fatal("to address valid faild", "address", paramTo)
	}

	if value, err := common.GetBigIntFromStr(paramValue); err != nil {
		log.Fatal("paramValue format not err", "paramValue", paramValue, "err", err)
	} else {
		if extra, err := InitExtra(); err != nil {
			log.Fatal("initExtra err", "err", err)
		} else {
			if txBuilder, err := BuildTx(paramFrom, paramTo, paramDenom, paramMemo, paramPublicKey, value, extra); err != nil {
				log.Fatal("BuildTx err", "err", err)
			} else {
				if signedTx, txHashBySign, err := MPCSignTransaction(txBuilder, extra); err != nil {
					log.Fatal("MPCSignTransaction err", "err", err)
				} else {
					if txHashBySendSend, err := SendTransaction(signedTx); err != nil {
						log.Fatal("SendTransaction err", "err", err)
					} else {
						log.Warn("success", "txHashBySign", txHashBySign, "txHashBySendSend", txHashBySendSend)
					}
				}
			}
		}
	}
}

func InitExtra() (extra *tokens.AllExtras, err error) {
	extra = &tokens.AllExtras{}
	if sequence, err := GetSeq(paramFrom); err != nil {
		return nil, err
	} else {
		extra.Sequence = sequence
	}
	if extra.AccountNum == nil {
		if accountNum, err := GetAccountNum(paramFrom); err != nil {
			return nil, err
		} else {
			extra.AccountNum = accountNum
		}
	}
	if extra.Gas == nil {
		extra.Gas = &cosmosHub.DefaultGasLimit
	}
	if extra.Fee == nil {
		extra.Fee = &cosmosHub.DefaultFee
	}
	return extra, nil
}

func GetSeq(address string) (*uint64, error) {
	var result *cosmosSDK.QueryAccountResponse
	restApi := url + cosmosSDK.AccountInfo + address
	if err := client.RPCGet(&result, restApi); err == nil {
		if result.Status == "ERROR" {
			return nil, fmt.Errorf(
				"GetBaseAccount error, address: %v, msg: %v",
				address, result.Msg)
		} else {
			if sequence, err := strconv.ParseUint(result.Account.Sequence, 10, 64); err == nil {
				return &sequence, nil
			} else {
				return nil, err
			}
		}
	} else {
		return nil, err
	}
}

func GetAccountNum(address string) (*uint64, error) {
	var result *cosmosSDK.QueryAccountResponse
	restApi := url + cosmosSDK.AccountInfo + address
	if err := client.RPCGet(&result, restApi); err == nil {
		if result.Status == "ERROR" {
			return nil, fmt.Errorf(
				"GetBaseAccount error, address: %v, msg: %v",
				address, result.Msg)
		} else {
			if accountNumber, err := strconv.ParseUint(result.Account.AccountNumber, 10, 64); err == nil {
				return &accountNumber, nil
			} else {
				return nil, err
			}
		}
	} else {
		return nil, err
	}
}

func MPCSignTransaction(rawTx interface{}, extra *tokens.AllExtras) (signedTx interface{}, txHash string, err error) {
	if txBulider, ok := rawTx.(cosmosClient.TxBuilder); !ok {
		return nil, txHash, errors.New("wrong raw tx param")
	} else {
		return SignTransactionWithPrivateKey(txBulider, paramPrivKey, extra)
	}
}

func SignTransactionWithPrivateKey(txBuilder cosmosClient.TxBuilder, privKey string, extra *tokens.AllExtras) (signedTx interface{}, txHash string, err error) {
	if ecPrikey, err := crypto.HexToECDSA(privKey); err != nil {
		return nil, "", err
	} else {
		ecPriv := &secp256k1.PrivKey{Key: ecPrikey.D.Bytes()}
		pubKey := ecPriv.PubKey()
		txConfig := NewTxConfig()
		handler := txConfig.SignModeHandler()
		signerData := cosmosSDK.BuildSignerData(paramFrom, "cosmoshub-4", *extra.AccountNum, *extra.Sequence, pubKey)

		// Generate the bytes to be signed.
		if signBytes, err := handler.GetSignBytes(signingTypes.SignMode_SIGN_MODE_DIRECT, signerData, txBuilder.GetTx()); err != nil {
			return nil, "", err
		} else {
			// Sign those bytes
			if signature, err := ecPriv.Sign(signBytes); err != nil {
				return nil, "", err
			} else {
				if len(signature) == crypto.SignatureLength {
					signature = signature[:crypto.SignatureLength-1]
				}

				if len(signature) != crypto.SignatureLength-1 {
					return nil, "", errors.New("wrong signature length")
				}

				if !pubKey.VerifySignature(signBytes, signature) {
					return nil, "", errors.New("wrong signature")
				}

				sig := cosmosSDK.BuildSignatures(pubKey, *extra.Sequence, signature)
				if err := txBuilder.SetSignatures(sig); err != nil {
					return nil, "", err
				}

				if err := txBuilder.GetTx().ValidateBasic(); err != nil {
					return nil, "", err
				}

				return GetSignTx(txBuilder.GetTx())
			}
		}
	}
}

func GetSignTx(tx signing.Tx) (signedTx []byte, txHash string, err error) {
	txconfig := NewTxConfig()
	if txBytes, err := txconfig.TxEncoder()(tx); err != nil {
		return nil, "", err
	} else {
		signedTx = []byte(base64.StdEncoding.EncodeToString(txBytes))
		txHash = fmt.Sprintf("%X", cosmosSDK.Sha256Sum(txBytes))
		log.Warn("GetSignTx", "signedTx", string(signedTx), "txHash", txHash)
		return signedTx, txHash, nil
	}
}

// SendTransaction send signed tx
func SendTransaction(signedTx interface{}) (txHash string, err error) {
	if txBytes, ok := signedTx.([]byte); !ok {
		return "", errors.New("wrong signed transaction type")
	} else {
		// use sync mode because block mode may rpc call timeout
		req := &cosmosSDK.BroadcastTxRequest{
			TxBytes: string(txBytes),
			Mode:    "BROADCAST_MODE_SYNC",
		}
		if txRes, err := BroadcastTx(req); err != nil {
			return "", err
		} else {
			var txResponse *cosmosSDK.BroadcastTxResponse
			if err := json.Unmarshal([]byte(txRes), &txResponse); err != nil {
				log.Warnf("Unmarshal BroadcastTxResponse err:%+v", err)
				return "", err
			}
			return txResponse.TxResponse.Txhash, nil
		}
	}
}

func BroadcastTx(req *cosmosSDK.BroadcastTxRequest) (string, error) {
	if data, err := json.Marshal(req); err != nil {
		return "", err
	} else {
		restApi := url + cosmosSDK.BroadTx
		if res, err := client.RPCRawPostWithTimeout(restApi, string(data), 60); err == nil {
			return res, nil
		} else {
			return "", err
		}
	}
}

func NewTxConfig() cosmosClient.TxConfig {
	interfaceRegistry := codecTypes.NewInterfaceRegistry()
	bankTypes.RegisterInterfaces(interfaceRegistry)
	cosmosSDK.PublicKeyRegisterInterfaces(interfaceRegistry)
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	return authTx.NewTxConfig(protoCodec, authTx.DefaultSignModes)
}

func BuildTx(
	from, to, denom, memo, publicKey string,
	amount *big.Int,
	extra *tokens.AllExtras,
) (cosmosClient.TxBuilder, error) {
	txconfig := NewTxConfig()
	txBuilder := txconfig.NewTxBuilder()
	msg := cosmosSDK.BuildSendMgs(from, to, denom, amount)
	if err := txBuilder.SetMsgs(msg); err != nil {
		return nil, err
	}
	txBuilder.SetMemo(memo)
	if fee, err := cosmosSDK.ParseCoinsFee(*extra.Fee); err != nil {
		return nil, err
	} else {
		txBuilder.SetFeeAmount(fee)
	}
	txBuilder.SetGasLimit(*extra.Gas)
	if pubKey, err := cosmosSDK.PubKeyFromStr(publicKey); err != nil {
		return nil, err
	} else {
		sig := cosmosSDK.BuildSignatures(pubKey, *extra.Sequence, nil)
		if err := txBuilder.SetSignatures(sig); err != nil {
			return nil, err
		}
	}
	if err := txBuilder.GetTx().ValidateBasic(); err != nil {
		return nil, err
	}
	return txBuilder, nil
}

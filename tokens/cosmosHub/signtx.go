package cosmosHub

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	if txBulider, ok := rawTx.(cosmosClient.TxBuilder); !ok {
		return nil, txHash, errors.New("wrong raw tx param")
	} else {
		if txBytes, err := cosmosSDK.GetTxDataBytes(txBulider); err != nil {
			return nil, txHash, err
		} else {
			mpcParams := params.GetMPCConfig(b.UseFastMPC)
			mpcPubkey := router.GetMPCPublicKey(args.From)
			if mpcPubkey == "" {
				return nil, "", tokens.ErrMissMPCPublicKey
			}
			if mpcParams.SignWithPrivateKey {
				priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
				return b.SignTransactionWithPrivateKey(txBulider, mpcPubkey, priKey, args)
			}
			jsondata, _ := json.Marshal(args.GetExtraArgs())
			msgContext := string(jsondata)

			txid := args.SwapID
			logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
			log.Info(logPrefix+"start", "txid", txid)

			mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
			msgHash := fmt.Sprintf("%X", Sha256Sum(txBytes))
			if keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, msgHash, msgContext); err != nil {
				return nil, "", err
			} else {
				if len(rsvs) != 1 {
					log.Warn("get sign status require one rsv but return many",
						"rsvs", len(rsvs), "keyID", keyID, "txid", txid)
					return nil, "", errors.New("get sign status require one rsv but return many")
				}

				rsv := rsvs[0]
				log.Trace(logPrefix+"get rsv signature success", "keyID", keyID, "txid", txid, "rsv", rsv)
				signature := common.FromHex(rsv)

				if len(signature) == crypto.SignatureLength {
					signature = signature[:crypto.SignatureLength-1]
				}

				if len(signature) != crypto.SignatureLength-1 {
					log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(signature), "want", crypto.SignatureLength)
					return nil, "", errors.New("wrong signature length")
				}

				if pubKey, err := cosmosSDK.PubKeyFromStr(mpcPubkey); err != nil {
					return nil, "", err
				} else {
					if !pubKey.VerifySignature(txBytes, signature) {
						log.Error("verify signature failed", "signBytes", common.ToHex(txBytes), "signature", signature)
						return nil, "", errors.New("wrong signature")
					}
					sig := cosmosSDK.BuildSignatures(pubKey, *args.Extra.Sequence, signature)
					if err := txBulider.SetSignatures(sig); err != nil {
						return nil, "", err
					}
					if err := txBulider.GetTx().ValidateBasic(); err != nil {
						return nil, "", err
					}

					if signBytes, err := cosmosSDK.GetTxDataBytes(txBulider); err != nil {
						return nil, "", err
					} else {
						signedTx = []byte(base64.StdEncoding.EncodeToString(signBytes))
						txHash = fmt.Sprintf("%X", Sha256Sum(signBytes))
						return signedTx, txHash, nil
					}
				}
			}
		}
	}
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(txBulider cosmosClient.TxBuilder, mpcPubkey string, privKey string, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	if ecPrikey, err := crypto.HexToECDSA(privKey); err != nil {
		return nil, "", err
	} else {
		ecPriv := &secp256k1.PrivKey{Key: ecPrikey.D.Bytes()}
		if txBytes, err := cosmosSDK.GetTxDataBytes(txBulider); err != nil {
			return nil, "", err
		} else {
			if signature, err := ecPriv.Sign(txBytes); err != nil {
				return nil, "", err
			} else {

				if len(signature) == crypto.SignatureLength {
					signature = signature[:crypto.SignatureLength-1]
				}

				if len(signature) != crypto.SignatureLength-1 {
					log.Error("wrong length of signature", "length", len(signature))
					return nil, "", errors.New("wrong signature length")
				}

				pubKey := ecPriv.PubKey()
				if !pubKey.VerifySignature(txBytes, signature) {
					log.Error("verify signature failed", "signBytes", common.ToHex(txBytes), "signature", signature)
					return nil, "", errors.New("wrong signature")
				}
				sig := cosmosSDK.BuildSignatures(pubKey, *args.Extra.Sequence, signature)
				if err := txBulider.SetSignatures(sig); err != nil {
					return nil, "", err
				}
				if err := txBulider.GetTx().ValidateBasic(); err != nil {
					return nil, "", err
				}

				if signBytes, err := cosmosSDK.GetTxDataBytes(txBulider); err != nil {
					return nil, "", err
				} else {
					signedTx = []byte(base64.StdEncoding.EncodeToString(signBytes))
					txHash = fmt.Sprintf("%X", Sha256Sum(signBytes))
					return signedTx, txHash, nil
				}
			}
		}
	}
}

// Sha256Sum returns the SHA256 of the data.
func Sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

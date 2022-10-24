package cosmosRouter

import (
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
	if buildRawTx, ok := rawTx.(*cosmosSDK.BuildRawTx); !ok {
		return nil, txHash, errors.New("wrong raw tx param")
	} else {
		txBuilder := *buildRawTx.TxBuilder
		mpcParams := params.GetMPCConfig(b.UseFastMPC)
		if mpcParams.SignWithPrivateKey {
			priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
			return b.SignTransactionWithPrivateKey(txBuilder, priKey, args)
		}

		mpcPubkey := router.GetMPCPublicKey(args.From)
		if mpcPubkey == "" {
			return nil, "", tokens.ErrMissMPCPublicKey
		}
		pubKey, err := cosmosSDK.PubKeyFromStr(mpcPubkey)
		if err != nil {
			return nil, txHash, err
		}
		if signBytes, err := b.CosmosRestClient.GetSignBytes(txBuilder, *args.Extra.AccountNum, *args.Extra.Sequence); err != nil {
			return nil, "", err
		} else {
			jsondata, _ := json.Marshal(args.GetExtraArgs())
			msgContext := string(jsondata)

			txid := args.SwapID
			logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
			log.Info(logPrefix+"start", "txid", txid)

			mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
			msgHash := fmt.Sprintf("%X", cosmosSDK.Sha256Sum(signBytes))
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

				if !pubKey.VerifySignature(signBytes, signature) {
					log.Error("verify signature failed", "signBytes", common.ToHex(signBytes), "signature", signature)
					return nil, "", errors.New("wrong signature")
				}
				sig := cosmosSDK.BuildSignatures(pubKey, *args.Extra.Sequence, signature)
				if err := txBuilder.SetSignatures(sig); err != nil {
					return nil, "", err
				}

				return b.CosmosRestClient.GetSignTx(txBuilder.GetTx())
			}
		}
	}
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(txBuilder cosmosClient.TxBuilder, privKey string, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	if ecPrikey, err := crypto.HexToECDSA(privKey); err != nil {
		return nil, "", err
	} else {
		ecPriv := &secp256k1.PrivKey{Key: ecPrikey.D.Bytes()}

		if signBytes, err := b.CosmosRestClient.GetSignBytes(txBuilder, *args.Extra.AccountNum, *args.Extra.Sequence); err != nil {
			return nil, "", err
		} else {
			if signature, err := ecPriv.Sign(signBytes); err != nil {
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
				if !pubKey.VerifySignature(signBytes, signature) {
					log.Error("verify signature failed", "signBytes", common.ToHex(signBytes), "signature", signature)
					return nil, "", errors.New("wrong signature")
				}
				sig := cosmosSDK.BuildSignatures(pubKey, *args.Extra.Sequence, signature)
				if err := txBuilder.SetSignatures(sig); err != nil {
					return nil, "", err
				}
				// if err := txBuilder.GetTx().ValidateBasic(); err != nil {
				// 	return nil, "", err
				// }

				return b.CosmosRestClient.GetSignTx(txBuilder.GetTx())
			}
		}
	}
}

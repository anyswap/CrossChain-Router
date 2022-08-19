package cardano

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	if txPath, ok := rawTx.(string); !ok {
		return nil, "", tokens.ErrWrongRawTx
	} else {
		mpcParams := params.GetMPCConfig(b.UseFastMPC)
		if txHash, err := b.getTxHash(txPath); err != nil {
			if mpcParams.SignWithPrivateKey {
				priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
				return b.SignTransactionWithPrivateKey(txHash, priKey)
			}
			mpcPubkey := router.GetMPCPublicKey(args.From)
			if mpcPubkey == "" {
				return nil, "", tokens.ErrMissMPCPublicKey
			}

			jsondata, _ := json.Marshal(args.GetExtraArgs())
			msgContext := string(jsondata)

			txid := args.SwapID
			logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
			log.Info(logPrefix+"start", "txid", txid)

			mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
			if keyID, rsvs, err := mpcConfig.DoSignOneED(mpcPubkey, txHash, msgContext); err != nil {
				return nil, "", err
			} else {
				if len(rsvs) != 1 {
					log.Warn("get sign status require one rsv but return many",
						"rsvs", len(rsvs), "keyID", keyID, "txid", txid)
					return nil, "", errors.New("get sign status require one rsv but return many")
				}

				rsv := rsvs[0]
				log.Trace(logPrefix+"get rsv signature success", "keyID", keyID, "txid", txid, "rsv", rsv)
				sig := common.FromHex(rsv)
				if len(sig) != ed25519.SignatureSize {
					log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(sig), "want", ed25519.SignatureSize)
					return nil, "", errors.New("wrong signature length")
				}

				if witnessPath, err := b.createWitness(args.From, sig); err != nil {
					return nil, "", err
				} else {
					if signTxPath, err := b.signTx(txPath, witnessPath); err != nil {
						return nil, "", err
					} else {
						return signTxPath, txHash, nil
					}
				}
			}
		} else {
			return nil, "", err
		}
	}
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx string, privKey string) (signTx interface{}, txHash string, err error) {
	return nil, "", tokens.ErrNotImplemented
}

func (b *Bridge) getTxHash(txPath string) (string, error) {
	return "", nil
}

func (b *Bridge) createWitness(mpc string, sig []byte) (string, error) {
	return "", nil
}

func (b *Bridge) signTx(rawTxPath, witnessPath string) (string, error) {
	return "", nil
}

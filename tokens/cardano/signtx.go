package cardano

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/fxamacker/cbor/v2"
)

const (
	WitnessPath   = "txDb/witness/"
	WitnessSuffix = ".witness"
	WitnessType   = "TxWitness AlonzoEra"
	SignedPath    = "txDb/signed/"
	SignedSuffix  = ".signed"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	if rawTransaction, ok := rawTx.(*RawTransaction); !ok {
		return nil, "", tokens.ErrWrongRawTx
	} else {
		txPath := RawPath + rawTransaction.OutFile + RawSuffix
		witnessPath := WitnessPath + rawTransaction.OutFile + WitnessSuffix
		signedPath := SignedPath + rawTransaction.OutFile + SignedSuffix

		mpcParams := params.GetMPCConfig(b.UseFastMPC)
		if txHash, err := CalcTxId(txPath); err != nil {
			return nil, "", err
		} else {
			mpcPubkey := router.GetMPCPublicKey(args.From)
			if mpcPubkey == "" {
				return nil, "", tokens.ErrMissMPCPublicKey
			}

			if mpcParams.SignWithPrivateKey {
				priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
				return b.SignTransactionWithPrivateKey(txPath, witnessPath, signedPath, txHash, mpcPubkey, priKey)
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

				if err := b.createWitness(witnessPath, mpcPubkey, sig); err != nil {
					return nil, "", err
				} else {
					if err := b.signTx(txPath, witnessPath, signedPath); err != nil {
						return nil, "", err
					} else {
						return &SignedTransaction{
							FilePath: signedPath,
							TxHash:   txHash,
						}, txHash, nil
					}
				}
			}
		}
	}
}

func CalcTxId(txPath string) (string, error) {
	cmdString := fmt.Sprintf(CalcTxIdCmd, txPath)
	if execRes, err := ExecCmd(cmdString, " "); err != nil {
		return "", err
	} else {
		return execRes[:len(execRes)-1], nil
	}
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(txPath, witnessPath, signedPath, txHash, mpcPubkey string, privKey string) (*SignedTransaction, string, error) {
	if edPrivKey, err := StringToPrivateKey(privKey); err != nil {
		return nil, "", err
	} else {
		if sig, err := edPrivKey.Sign(rand.Reader, []byte(txHash)[:], crypto.Hash(0)); err != nil {
			return nil, "", err
		} else {
			if err := b.createWitness(witnessPath, mpcPubkey, sig); err != nil {
				return nil, "", err
			} else {
				if err := b.signTx(txPath, witnessPath, signedPath); err != nil {
					return nil, "", err
				} else {
					return &SignedTransaction{
						FilePath: signedPath,
						TxHash:   txHash,
					}, txHash, nil
				}
			}
		}
	}
}

func (b *Bridge) createWitness(witnessPath, mpcPublicKey string, sig []byte) error {
	var str [2]interface{}
	str[0] = 0
	if publicKey, err := hex.DecodeString(mpcPublicKey); err != nil {
		return err
	} else {
		str[1] = [2][]byte{publicKey, sig}
		if res, err := cbor.Marshal(str); err != nil {
			return err
		} else {
			dataMap := make(map[string]string)
			dataMap["type"] = WitnessType
			dataMap["description"] = ""
			dataMap["cborHex"] = hex.EncodeToString(res)
			file, err := os.Create(witnessPath)
			if err != nil {
				return err
			}
			defer file.Close()
			if err := json.NewEncoder(file).Encode(dataMap); err != nil {
				return err
			} else {
				return nil
			}
		}
	}
}

func (b *Bridge) signTx(rawTxPath, witnessPath, signedPath string) error {
	cmdString := fmt.Sprintf(AssembleCmd, rawTxPath, witnessPath, signedPath)
	if _, err := ExecCmd(cmdString, " "); err != nil {
		return err
	} else {
		return nil
	}
}

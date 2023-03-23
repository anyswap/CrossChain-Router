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
	"github.com/btcsuite/btcutil/bech32"
	cardanosdk "github.com/echovl/cardano-go"
	"github.com/echovl/cardano-go/crypto"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	if rawTransaction, ok := rawTx.(*RawTransaction); !ok {
		return nil, "", tokens.ErrWrongRawTx
	} else {
		tx, err := b.CreateRawTx(rawTransaction, b.GetRouterContract(""))
		if err != nil {
			return nil, "", err
		}

		mpcParams := params.GetMPCConfig(b.UseFastMPC)
		if mpcParams.SignWithPrivateKey {
			priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
			return b.SignTransactionWithPrivateKey(tx, rawTransaction, args, priKey)
		}

		signingMsg, err := tx.Hash()
		if err != nil {
			return nil, "", err
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
		if keyID, rsvs, err := mpcConfig.DoSignOneED(mpcPubkey, signingMsg.String(), msgContext); err != nil {
			log.Info(logPrefix+"failed", "keyID", keyID, "txid", txid, "err", err)
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

			pubStr, _ := bech32.EncodeFromBase256("addr_vk", common.FromHex(mpcPubkey))
			pubKey, _ := crypto.NewPubKey(pubStr)
			b.AppendSignature(tx, pubKey, sig)

			cacheAssetsMap := rawTransaction.TxOuts[args.From]
			txInputs := rawTransaction.TxIns
			txIndex := rawTransaction.TxIndex
			return &SignedTransaction{
				TxIns:     txInputs,
				TxHash:    signingMsg.String(),
				TxIndex:   txIndex,
				AssetsMap: cacheAssetsMap,
				Tx:        tx,
			}, signingMsg.String(), nil
		}
	}
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(tx *cardanosdk.Tx, rawTransaction *RawTransaction, args *tokens.BuildTxArgs, privKey string) (*SignedTransaction, string, error) {
	sk, err := crypto.NewPrvKey(privKey)
	if err != nil {
		return nil, "", err
	}
	txHash, err := tx.Hash()
	if err != nil {
		return nil, "", err
	}
	b.AppendSignature(tx, sk.PubKey(), sk.Sign(txHash))

	cacheAssetsMap := rawTransaction.TxOuts[args.From]
	txInputs := rawTransaction.TxIns
	txIndex := rawTransaction.TxIndex

	return &SignedTransaction{
		TxIns:     txInputs,
		TxHash:    txHash.String(),
		TxIndex:   txIndex,
		AssetsMap: cacheAssetsMap,
		Tx:        tx,
	}, txHash.String(), nil
}

func (b *Bridge) AppendSignature(tx *cardanosdk.Tx, pubKey crypto.PubKey, signature []byte) {
	newVKeyWitnessSet := []cardanosdk.VKeyWitness{}
	for _, vKeyWitness := range tx.WitnessSet.VKeyWitnessSet {
		if vKeyWitness.VKey.String() == b.FakePrikey.PubKey().String() {
			newVKeyWitnessSet = append(newVKeyWitnessSet, cardanosdk.VKeyWitness{
				VKey:      pubKey,
				Signature: signature,
			})
		} else {
			newVKeyWitnessSet = append(newVKeyWitnessSet, vKeyWitness)
		}
	}
	tx.WitnessSet.VKeyWitnessSet = newVKeyWitnessSet
}

// func (b *Bridge) CreateWitness(witnessPath, mpcPublicKey string, sig []byte) error {
// 	var str [2]interface{}
// 	str[0] = 0
// 	if publicKey, err := hex.DecodeString(mpcPublicKey); err != nil {
// 		return err
// 	} else {
// 		str[1] = [2][]byte{publicKey, sig}
// 		if res, err := cbor.Marshal(str); err != nil {
// 			return err
// 		} else {
// 			dataMap := make(map[string]string)
// 			dataMap["type"] = WitnessType
// 			dataMap["description"] = ""
// 			dataMap["cborHex"] = hex.EncodeToString(res)
// 			file, err := os.Create(witnessPath)
// 			if err != nil {
// 				return err
// 			}
// 			defer file.Close()
// 			if err := json.NewEncoder(file).Encode(dataMap); err != nil {
// 				return err
// 			}
// 			return nil
// 		}
// 	}
// }

func (b *Bridge) MPCSignSwapTransaction(rawTx interface{}, args *tokens.BuildTxArgs, bind, toChainId string) (signTx interface{}, txHash string, err error) {
	if rawTransaction, ok := rawTx.(*RawTransaction); !ok {
		return nil, "", tokens.ErrWrongRawTx
	} else {
		tx, err := b.CreateSwapoutRawTx(rawTransaction, b.GetRouterContract(""), bind, toChainId)
		if err != nil {
			return nil, "", err
		}

		mpcParams := params.GetMPCConfig(b.UseFastMPC)
		if mpcParams.SignWithPrivateKey {
			priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
			return b.SignTransactionWithPrivateKey(tx, rawTransaction, args, priKey)
		}

		signingMsg, err := tx.Hash()
		if err != nil {
			return nil, "", err
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
		if keyID, rsvs, err := mpcConfig.DoSignOneED(mpcPubkey, signingMsg.String(), msgContext); err != nil {
			log.Info(logPrefix+"failed", "keyID", keyID, "txid", txid, "err", err)
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

			pubStr, _ := bech32.EncodeFromBase256("addr_vk", common.FromHex(mpcPubkey))
			pubKey, _ := crypto.NewPubKey(pubStr)
			b.AppendSignature(tx, pubKey, sig)

			cacheAssetsMap := rawTransaction.TxOuts[args.From]
			txInputs := rawTransaction.TxIns
			txIndex := rawTransaction.TxIndex
			return &SignedTransaction{
				TxIns:     txInputs,
				TxHash:    txHash,
				TxIndex:   txIndex,
				AssetsMap: cacheAssetsMap,
				Tx:        tx,
			}, txHash, nil
		}
	}
}

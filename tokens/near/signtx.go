package near

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/mr-tron/base58"
	"github.com/near/borsh-go"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*RawTransaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}

	mpcParams := params.GetMPCConfig(b.UseFastMPC)
	if mpcParams.SignWithPrivateKey {
		priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
		return b.SignTransactionWithPrivateKey(rawTx, priKey)
	}

	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return nil, "", tokens.ErrMissMPCPublicKey
	}
	nearPubkey, err := PublicKeyFromString(mpcPubkey)
	if err != nil {
		return nil, "", err
	}
	mpcSignPubkey := hex.EncodeToString(nearPubkey.Bytes())

	buf, err := borsh.Serialize(*tx)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(buf)

	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid)

	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneED(mpcSignPubkey, common.ToHex(hash[:]), msgContext)
	if err != nil {
		log.Info(logPrefix+"failed", "keyID", keyID, "txid", txid, "err", err)
		return nil, "", err
	}

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

	var signature Signature
	signature.KeyType = ED25519
	copy(signature.Data[:], sig)

	var stx SignedTransaction
	stx.Transaction = *tx
	stx.Signature = signature

	txHash = base58.Encode(hash[:])

	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash)
	return &stx, txHash, nil
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key string
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	tx := rawTx.(*RawTransaction)
	log.Warn("SignTransactionWithPrivateKey", "privKey", privKey)
	edPrivKey, err := StringToPrivateKey(privKey)
	if err != nil {
		return nil, "", err
	}
	return signTransaction(tx, edPrivKey)
}

func signTransaction(tx *RawTransaction, privKey *ed25519.PrivateKey) (signedTx *SignedTransaction, txHash string, err error) {
	buf, err := borsh.Serialize(*tx)
	if err != nil {
		return nil, "", err
	}

	hash := sha256.Sum256(buf)
	sig, err := privKey.Sign(rand.Reader, hash[:], crypto.Hash(0))
	if err != nil {
		return nil, "", err
	}

	var signature Signature
	signature.KeyType = ED25519
	copy(signature.Data[:], sig)

	var stx SignedTransaction
	stx.Transaction = *tx
	stx.Signature = signature

	txHash = base58.Encode(hash[:])

	log.Info("sign tx success", "txhash", txHash)

	return &stx, txHash, nil
}

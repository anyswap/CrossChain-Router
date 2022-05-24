package reef

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	crypto1 "github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/anyswap/CrossChain-Router/v3/types"

	substratetypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"golang.org/x/crypto/blake2b"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*Extrinsic)
	if !ok {
		return nil, "", errors.New("[sign] wrong raw tx param")
	}
	if err != nil {
		return nil, "", err
	}

	mpcPubkey := router.GetMPCPublicKey(args.EVMFrom)
	if mpcPubkey == "" {
		return nil, "", tokens.ErrMissMPCPublicKey
	}

	signerPubKey, err := substratetypes.NewMultiAddressFromHexAccountID(args.From)
	if err != nil {
		return nil, "", err
	}

	mb, err := substratetypes.Encode(tx.Method)
	if err != nil { return nil, "", err }

	payload := substratetypes.ExtrinsicPayloadV4{
		ExtrinsicPayloadV3: substratetypes.ExtrinsicPayloadV3{
			Method:      mb,
			Era:         tx.SignatureOptions.Era,
			Nonce:       tx.SignatureOptions.Nonce,
			Tip:         tx.SignatureOptions.Tip,
			SpecVersion: tx.SignatureOptions.SpecVersion,
			GenesisHash: tx.SignatureOptions.GenesisHash,
			BlockHash:   tx.SignatureOptions.BlockHash,
		},
		TransactionVersion: tx.SignatureOptions.TransactionVersion,
	}

	bz, err := substratetypes.Encode(payload)
	if err != nil { return nil, "", err }
	hash := blake2b.Sum256(bz)

	msgHash := fmt.Sprintf("%x", hash[:])
	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid, "msghash", msgHash)

	keyID, rsvs, err := mpc.DoSignOneED(mpcPubkey, common.ToHex(hash[:]), msgContext)
	if err != nil {
		return nil, "", err
	}
	log.Info(logPrefix+"finished", "keyID", keyID, "txid", txid, "msghash", msgHash)

	if len(rsvs) != 1 {
		log.Warn("get sign status require one rsv but return many",
			"rsvs", len(rsvs), "keyID", keyID, "txid", txid)
		return nil, "", errors.New("get sign status require one rsv but return many")
	}

	rsv := rsvs[0]
	log.Trace(logPrefix+"get rsv signature success", "keyID", keyID, "txid", txid, "rsv", rsv)
	sig := common.FromHex(rsv)
	
	if len(sig) != crypto1.SignatureLength {
		log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(sig), "want", crypto1.SignatureLength)
		return nil, "", errors.New("wrong signature length")
	}

	signature := substratetypes.NewSignature(sig)

	extSig := substratetypes.ExtrinsicSignatureV4{
		Signer:    signerPubKey,
		Signature: substratetypes.MultiSignature{IsEd25519: false, AsEd25519: signature},
		Era:       tx.SignatureOptions.Era,
		Nonce:     tx.SignatureOptions.Nonce,
		Tip:       tx.SignatureOptions.Tip,
	}

	tx.Signature = extSig

	// mark the extrinsic as signed
	tx.Version |= substratetypes.ExtrinsicBitSigned

	bz2, err := substratetypes.Encode(payload)
	if err != nil { return nil, "", err }
	signedHash := blake2b.Sum256(bz2)
	txHash = fmt.Sprintf("%#x", signedHash[:])

	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash, "nonce", tx.SignatureOptions.Nonce)
	return tx, txHash, nil
}

// GetSignedTxHashOfKeyID get signed tx hash by keyID (called by oracle)
func (b *Bridge) GetSignedTxHashOfKeyID(sender, keyID string, rawTx interface{}) (txHash string, err error) {
	tx, ok := rawTx.(*Extrinsic)
	if !ok {
		return "", errors.New("wrong raw tx of keyID " + keyID)
	}
	rsvs, err := mpc.GetSignStatusByKeyID(keyID)
	if err != nil {
		return "", err
	}
	if len(rsvs) != 1 {
		return "", errors.New("wrong number of rsvs of keyID " + keyID)
	}

	rsv := rsvs[0]
	sig := common.FromHex(rsv)
	if len(sig) != crypto1.SignatureLength {
		return "", errors.New("wrong signature of keyID " + keyID)
	}

	signature := substratetypes.NewSignature(sig)

	signerPubKey, err := substratetypes.NewMultiAddressFromHexAccountID(sender)
	if err != nil {
		return "", err
	}

	mb, err := substratetypes.Encode(tx.Method)
	if err != nil { return "", err }

	payload := substratetypes.ExtrinsicPayloadV4{
		ExtrinsicPayloadV3: substratetypes.ExtrinsicPayloadV3{
			Method:      mb,
			Era:         tx.SignatureOptions.Era,
			Nonce:       tx.SignatureOptions.Nonce,
			Tip:         tx.SignatureOptions.Tip,
			SpecVersion: tx.SignatureOptions.SpecVersion,
			GenesisHash: tx.SignatureOptions.GenesisHash,
			BlockHash:   tx.SignatureOptions.BlockHash,
		},
		TransactionVersion: tx.SignatureOptions.TransactionVersion,
	}

	extSig := substratetypes.ExtrinsicSignatureV4{
		Signer:    signerPubKey,
		Signature: substratetypes.MultiSignature{IsEd25519: false, AsEd25519: signature},
		Era:       tx.SignatureOptions.Era,
		Nonce:     tx.SignatureOptions.Nonce,
		Tip:       tx.SignatureOptions.Tip,
	}

	tx.Signature = extSig

	// mark the extrinsic as signed
	tx.Version |= substratetypes.ExtrinsicBitSigned

	bz2, err := substratetypes.Encode(payload)
	if err != nil { return "", err }
	signedHash := blake2b.Sum256(bz2)
	txHash = fmt.Sprintf("%#x", signedHash[:])

	return txHash, nil
}

// SignTransactionWithPrivateKey sign tx with private key (use for testing)
func (b *Bridge) SignTransactionWithPrivateKey(sender string, rawTx interface{}, privKey *ed25519.PrivateKey) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*Extrinsic)
	if !ok {
		return nil, "", errors.New("wrong raw tx param")
	}

	mb, err := substratetypes.Encode(tx.Method)
	if err != nil { return nil, "", err }

	payload := substratetypes.ExtrinsicPayloadV4{
		ExtrinsicPayloadV3: substratetypes.ExtrinsicPayloadV3{
			Method:      mb,
			Era:         tx.SignatureOptions.Era,
			Nonce:       tx.SignatureOptions.Nonce,
			Tip:         tx.SignatureOptions.Tip,
			SpecVersion: tx.SignatureOptions.SpecVersion,
			GenesisHash: tx.SignatureOptions.GenesisHash,
			BlockHash:   tx.SignatureOptions.BlockHash,
		},
		TransactionVersion: tx.SignatureOptions.TransactionVersion,
	}

	bz, err := substratetypes.Encode(payload)
	if err != nil { return nil, "", err }
	hash := blake2b.Sum256(bz)

	sig, err := privKey.Sign(rand.Reader, hash[:], crypto.Hash(0))
	signature := substratetypes.NewSignature(sig)

	signerPubKey, err := substratetypes.NewMultiAddressFromHexAccountID(sender)
	if err != nil {
		return nil, "", err
	}

	extSig := substratetypes.ExtrinsicSignatureV4{
		Signer:    signerPubKey,
		Signature: substratetypes.MultiSignature{IsEd25519: false, AsEd25519: signature},
		Era:       tx.SignatureOptions.Era,
		Nonce:     tx.SignatureOptions.Nonce,
		Tip:       tx.SignatureOptions.Tip,
	}

	tx.Signature = extSig

	// mark the extrinsic as signed
	tx.Version |= substratetypes.ExtrinsicBitSigned

	bz2, err := substratetypes.Encode(payload)
	if err != nil { return nil, "", err }
	signedHash := blake2b.Sum256(bz2)
	txHash = fmt.Sprintf("%#x", signedHash[:])

	log.Info(b.ChainConfig.BlockChain+" SignTransaction success", "txhash", txHash, "nonce", tx.SignatureOptions.Nonce)
	return tx, txHash, err
}

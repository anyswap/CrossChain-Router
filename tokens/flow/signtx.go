package flow

import (
	"encoding/json"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	sdk "github.com/onflow/flow-go-sdk"
	fcrypto "github.com/onflow/flow-go-sdk/crypto"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*sdk.Transaction)
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

	message := tx.EnvelopeMessage()
	message = append(sdk.TransactionDomainTag[:], message...)
	hasher, _ := fcrypto.NewHasher(fcrypto.SHA3_256)
	hash := hasher.ComputeHash(message)
	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid)

	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	mpcRealPubkey, err := b.PubKeyToMpcPubKey(mpcPubkey)
	if err != nil {
		return nil, "", err
	}
	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcRealPubkey, common.ToHex(hash[:]), msgContext)
	if err != nil {
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
	if len(sig) != crypto.SignatureLength {
		log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(sig), "want", crypto.SignatureLength)
		return nil, "", errors.New("wrong signature length")
	}

	tx.AddEnvelopeSignature(tx.Payer, tx.ProposalKey.KeyIndex, sig[:64])

	txHash = tx.ID().String()
	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash, "nonce", tx.ProposalKey.SequenceNumber)
	return tx, txHash, err
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key string
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	ecPrikey, err := fcrypto.DecodePrivateKeyHex(fcrypto.ECDSA_P256, privKey)
	if err != nil {
		return nil, "", err
	}
	return signTransaction(rawTx, ecPrikey)
}

func signTransaction(tx interface{}, privKey fcrypto.PrivateKey) (signedTx interface{}, txHash string, err error) {
	rawTx := tx.(*sdk.Transaction)
	keySigner, err := fcrypto.NewInMemorySigner(privKey, fcrypto.SHA3_256)
	if err != nil {
		return nil, "", err
	}
	err = rawTx.SignEnvelope(rawTx.Payer, rawTx.ProposalKey.KeyIndex, keySigner)
	if err != nil {
		return nil, "", err
	}

	return rawTx, rawTx.ID().String(), nil
}

package stellar

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/btcsuite/btcd/btcec"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

func (b *Bridge) verifyTransactionWithArgs(tx *txnbuild.Transaction, args *tokens.BuildTxArgs) error {
	if len(tx.Operations()) <= 0 {
		return fmt.Errorf("not a payment transaction")
	}

	op := tx.Operations()[0].(*txnbuild.Payment)

	to := op.Destination

	checkReceiver := args.Bind
	if !strings.EqualFold(to, checkReceiver) {
		return fmt.Errorf("[sign] verify tx receiver failed")
	}
	return nil
}

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*txnbuild.Transaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}
	err = b.verifyTransactionWithArgs(tx, args)
	if err != nil {
		log.Warn("Verify transaction failed", "error", err)
		return nil, "", err
	}

	mpcParams := params.GetMPCConfig(b.UseFastMPC)
	if mpcParams.SignWithPrivateKey {
		priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
		return b.SignTransactionWithPrivateKey(rawTx, priKey)
	}

	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)
	txHashBeforeSign, _ := tx.HashHex(b.NetworkStr)

	txMsg, err := network.HashTransactionInEnvelope(tx.ToXDR(), b.NetworkStr)
	if err != nil {
		return nil, "", err
	}

	pubkeyStr := router.GetMPCPublicKey(args.From)
	pubkey := common.FromHex(pubkeyStr)

	var keyID string
	var rsvs []string

	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	if isEd25519Pubkey(pubkey) {
		// mpc ed public key has no 0xed prefix
		signPubKey := pubkeyStr[2:]
		// the real sign content is (signing prefix + msg)
		// when we hex encoding here, the mpc should do hex decoding there.
		signContent := common.ToHex(txMsg[:])
		keyID, rsvs, err = mpcConfig.DoSignOneED(signPubKey, signContent, msgContext)
	} else {
		return nil, "", fmt.Errorf("stellar not support ec privatekey, publickey: %v", pubkey)
	}

	if err != nil {
		return nil, "", err
	}
	log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction finished", "keyID", keyID, "txid", args.SwapID)

	if len(rsvs) != 1 {
		return nil, "", fmt.Errorf("get sign status require one rsv but have %v (keyID = %v)", len(rsvs), keyID)
	}

	rsv := rsvs[0]
	log.Trace(b.ChainConfig.BlockChain+" MPCSignTransaction get rsv success", "keyID", keyID, "rsv", rsv)

	sig := rsvToSig(rsv, true)

	pubkeyAddr, _ := b.PublicKeyToAddress(pubkeyStr)
	pubkeyKeyPair := keypair.MustParseAddress(pubkeyAddr)

	err = pubkeyKeyPair.Verify(txMsg[:], sig)
	if err != nil {
		return nil, "", fmt.Errorf("stellar verify signature error : %v", err)
	}

	signedTx, err := MakeSignedTransaction(pubkeyKeyPair, sig, tx)
	if err != nil {
		return signedTx, "", err
	}

	txhash, err := signedTx.HashHex(b.NetworkStr)

	if txHashBeforeSign != txhash {
		return nil, "", fmt.Errorf("stellar verify signature error : %v %v", txHashBeforeSign, txhash)
	}

	return signedTx, txhash, err
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signTx interface{}, txHash string, err error) {

	sourceKP := keypair.MustParseFull(privKey)

	return b.SignTransactionWithStellarKey(rawTx, sourceKP)
}

// SignTransactionWithStellarKey sign tx with stellar key
func (b *Bridge) SignTransactionWithStellarKey(rawTx interface{}, key *keypair.Full) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*txnbuild.Transaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}
	tx, err = tx.Sign(b.NetworkStr, key)
	if err != nil {
		return nil, "", err
	}
	txHash, _ = tx.HashHex(b.NetworkStr)
	return tx, txHash, nil
}

// MakeSignedTransaction make signed transaction
func MakeSignedTransaction(pubkey *keypair.FromAddress, sig []byte, tx *txnbuild.Transaction) (signedTx *txnbuild.Transaction, err error) {
	decoratedSignature := xdr.NewDecoratedSignature(sig, pubkey.Hint())
	return tx.AddSignatureDecorated(decoratedSignature)
}

func isEd25519Pubkey(pubkey []byte) bool {
	return len(pubkey) == ed25519.PublicKeySize+1 && pubkey[0] == 0xED
}

func rsvToSig(rsv string, isEd bool) []byte {
	if isEd {
		return common.FromHex(rsv)
	}
	b, _ := hex.DecodeString(rsv)
	rx := hex.EncodeToString(b[:32])
	sx := hex.EncodeToString(b[32:64])
	r, _ := new(big.Int).SetString(rx, 16)
	s, _ := new(big.Int).SetString(sx, 16)
	signature := &btcec.Signature{
		R: r,
		S: s,
	}
	return signature.Serialize()
}

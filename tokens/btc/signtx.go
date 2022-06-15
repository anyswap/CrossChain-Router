package btc

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*txauthor.AuthoredTx)
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

	return b.DcrmSignTransaction(rawTx, args)
	// jsondata, _ := json.Marshal(args.GetExtraArgs())
	// msgContext := string(jsondata)
	// msgHash, msg, err := data.SigningHash(tx)
	// if err != nil {
	// 	return nil, "", fmt.Errorf("get transaction signing hash failed: %w", err)
	// }
	// msg = append(tx.SigningPrefix().Bytes(), msg...)

	// pubkeyStr := router.GetMPCPublicKey(args.From)
	// pubkey := common.FromHex(pubkeyStr)
	// isEd := isEd25519Pubkey(pubkey)

	// var keyID string
	// var rsvs []string

	// mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	// if isEd {
	// 	// mpc ed public key has no 0xed prefix
	// 	signPubKey := pubkeyStr[2:]
	// 	// the real sign content is (signing prefix + msg)
	// 	// when we hex encoding here, the mpc should do hex decoding there.
	// 	signContent := common.ToHex(msg)
	// 	keyID, rsvs, err = mpcConfig.DoSignOneED(signPubKey, signContent, msgContext)
	// } else {
	// 	signPubKey := pubkeyStr
	// 	signContent := msgHash.String()
	// 	keyID, rsvs, err = mpcConfig.DoSignOneEC(signPubKey, signContent, msgContext)
	// }

	// if err != nil {
	// 	return nil, "", err
	// }
	// log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction finished", "keyID", keyID, "txid", args.SwapID)

	// if len(rsvs) != 1 {
	// 	return nil, "", fmt.Errorf("get sign status require one rsv but have %v (keyID = %v)", len(rsvs), keyID)
	// }

	// rsv := rsvs[0]
	// log.Trace(b.ChainConfig.BlockChain+" MPCSignTransaction get rsv success", "keyID", keyID, "rsv", rsv)

	// sig := rsvToSig(rsv, isEd)
	// valid, err := rcrypto.Verify(pubkey, msgHash.Bytes(), msg, sig)
	// if !valid || err != nil {
	// 	return nil, "", fmt.Errorf("verify signature error (valid: %v): %v", valid, err)
	// }

	// signedTx, err := MakeSignedTransaction(pubkey, rsv, rawTx)
	// if err != nil {
	// 	return signedTx, "", err
	// }

	// txhash := signedTx.GetHash().String()

	// return signedTx, txhash, nil
}

func (b *Bridge) verifyTransactionWithArgs(tx *txauthor.AuthoredTx, args *tokens.BuildTxArgs) error {
	checkReceiver := args.Bind
	payToReceiverScript, err := b.GetPayToAddrScript(checkReceiver)
	if err != nil {
		return err
	}
	isRightReceiver := false
	for _, out := range tx.Tx.TxOut {
		if bytes.Equal(out.PkScript, payToReceiverScript) {
			isRightReceiver = true
			break
		}
	}
	if !isRightReceiver {
		return fmt.Errorf("[sign] verify tx receiver failed")
	}
	return nil
}

// DcrmSignTransaction dcrm sign raw tx
func (b *Bridge) DcrmSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	authoredTx, ok := rawTx.(*txauthor.AuthoredTx)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}

	err = b.verifyTransactionWithArgs(authoredTx, args)
	if err != nil {
		return nil, "", err
	}
	pubkeyStr := router.GetMPCPublicKey(args.From)
	cPkData, err := b.GetCompressedPublicKey(pubkeyStr, false)
	if err != nil {
		return nil, "", err
	}

	var (
		msgHashes    []string
		rsvs         []string
		sigScripts   [][]byte
		hasP2shInput bool
		sigHash      []byte
	)

	for i, preScript := range authoredTx.PrevScripts {
		sigScript := preScript
		if b.IsPayToScriptHash(preScript) {
			sigScript, err = b.getRedeemScriptByOutputScrpit(preScript)
			if err != nil {
				return nil, "", err
			}
			hasP2shInput = true
		}

		sigHash, err = b.CalcSignatureHash(sigScript, authoredTx.Tx, i)
		if err != nil {
			return nil, "", err
		}
		msgHash := hex.EncodeToString(sigHash)
		msgHashes = append(msgHashes, msgHash)
		sigScripts = append(sigScripts, sigScript)
	}
	if !hasP2shInput {
		sigScripts = nil
	}

	rsvs, err = b.DcrmSignMsgHash(msgHashes, args)
	if err != nil {
		return nil, "", err
	}

	return b.MakeSignedTransaction(authoredTx, msgHashes, rsvs, sigScripts, cPkData)
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key string
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	ecPrikey, err := crypto.HexToECDSA(privKey)
	if err != nil {
		return nil, "", err
	}
	return b.signTransaction(rawTx, ecPrikey)
}

func (b *Bridge) signTransaction(tx interface{}, privKey *ecdsa.PrivateKey) (signedTx interface{}, txHash string, err error) {
	authoredTx, ok := tx.(*txauthor.AuthoredTx)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}

	var (
		msgHashes    []string
		rsvs         []string
		sigScripts   [][]byte
		hasP2shInput bool
	)

	for i, preScript := range authoredTx.PrevScripts {
		sigScript := preScript
		if b.IsPayToScriptHash(preScript) {
			sigScript, err = b.getRedeemScriptByOutputScrpit(preScript)
			if err != nil {
				return nil, "", err
			}
			hasP2shInput = true
		}

		sigHash, err := b.CalcSignatureHash(sigScript, authoredTx.Tx, i)
		if err != nil {
			return nil, "", err
		}
		msgHash := hex.EncodeToString(sigHash)
		msgHashes = append(msgHashes, msgHash)
		sigScripts = append(sigScripts, sigScript)
	}
	if !hasP2shInput {
		sigScripts = nil
	}

	for _, msgHash := range msgHashes {
		rsv, errf := b.SignWithECDSA(privKey, common.FromHex(msgHash))
		if errf != nil {
			return nil, "", errf
		}
		rsvs = append(rsvs, rsv)
	}

	cPkData := b.GetPublicKeyFromECDSA(privKey, true)
	return b.MakeSignedTransaction(authoredTx, msgHashes, rsvs, sigScripts, cPkData)
}

// SignWithECDSA sign with ecdsa private key
func (b *Bridge) SignWithECDSA(privKey *ecdsa.PrivateKey, msgHash []byte) (rsv string, err error) {
	signature, err := (*btcec.PrivateKey)(privKey).Sign(msgHash)
	if err != nil {
		return "", err
	}
	rr := fmt.Sprintf("%064X", signature.R)
	ss := fmt.Sprintf("%064X", signature.S)
	rsv = fmt.Sprintf("%s%s00", rr, ss)
	return rsv, nil
}

// GetPublicKeyFromECDSA get public key from ecdsa private key
func (b *Bridge) GetPublicKeyFromECDSA(privKey *ecdsa.PrivateKey, compressed bool) []byte {
	if compressed {
		return (*btcec.PublicKey)(&privKey.PublicKey).SerializeCompressed()
	}
	return (*btcec.PublicKey)(&privKey.PublicKey).SerializeUncompressed()
}

// MakeSignedTransaction make signed tx
func (b *Bridge) MakeSignedTransaction(authoredTx *txauthor.AuthoredTx, msgHash, rsv []string, sigScripts [][]byte, cPkData []byte) (signedTx interface{}, txHash string, err error) {
	if len(cPkData) == 0 {
		return nil, "", errors.New("empty public key data")
	}
	err = checkEqualLength(authoredTx, msgHash, rsv, sigScripts)
	if err != nil {
		return nil, "", err
	}
	log.Info(b.ChainConfig.BlockChain+" Bridge MakeSignedTransaction", "msghash", msgHash, "count", len(msgHash))

	for i, txin := range authoredTx.Tx.TxIn {
		signData, ok := b.getSigDataFromRSV(rsv[i])
		if !ok {
			return nil, "", errors.New("wrong RSV data")
		}

		sigScript, err := b.GetSigScript(sigScripts, authoredTx.PrevScripts[i], signData, cPkData, i)
		if err != nil {
			return nil, "", err
		}
		txin.SignatureScript = sigScript
	}
	txHash = authoredTx.Tx.TxHash().String()
	log.Info(b.ChainConfig.BlockChain+" MakeSignedTransaction success", "txhash", txHash)
	return authoredTx, txHash, nil
}

func checkEqualLength(authoredTx *txauthor.AuthoredTx, msgHash, rsv []string, sigScripts [][]byte) error {
	txIn := authoredTx.Tx.TxIn
	if len(txIn) != len(msgHash) {
		return errors.New("mismatch number of msghashes and tx inputs")
	}
	if len(txIn) != len(rsv) {
		return errors.New("mismatch number of signatures and tx inputs")
	}
	if sigScripts != nil && len(sigScripts) != len(txIn) {
		return errors.New("mismatch number of signatures scripts and tx inputs")
	}
	return nil
}

// DcrmSignMsgHash dcrm sign msg hash
func (b *Bridge) DcrmSignMsgHash(msgHash []string, args *tokens.BuildTxArgs) (rsv []string, err error) {
	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := []string{string(jsondata)}
	pubkeyStr := router.GetMPCPublicKey(args.From)
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)

	log.Info(b.ChainConfig.BlockChain+" DcrmSignTransaction start", "msgContext", msgContext, "txid", args.SwapID)
	keyID, rsv, err := mpcConfig.DoSign("EC256K1", pubkeyStr, msgHash, msgContext)
	if err != nil {
		return nil, err
	}
	log.Info(b.ChainConfig.BlockChain+" DcrmSignTransaction finished", "keyID", keyID, "msghash", msgHash, "txid", args.SwapID)

	if len(rsv) != len(msgHash) {
		return nil, fmt.Errorf("get sign status require %v rsv but have %v (keyID = %v)", len(msgHash), len(rsv), keyID)
	}

	rsv, err = b.adjustRsvOrders(rsv, msgHash, pubkeyStr)
	if err != nil {
		return nil, err
	}

	log.Trace(b.ChainConfig.BlockChain+" DcrmSignTransaction get rsv success", "keyID", keyID, "txid", args.SwapID, "rsv", rsv)
	return rsv, nil
}


func (b *Bridge) adjustRsvOrders(rsvs, msgHashes []string, fromPublicKey string) (newRsvs []string, err error) {
	if len(rsvs) <= 1 {
		return rsvs, nil
	}
	fromPubkeyData, err := b.GetCompressedPublicKey(fromPublicKey, false)
	matchedRsvMap := make(map[string]struct{})
	var cPkData []byte
	for _, msgHash := range msgHashes {
		matched := false
		for _, rsv := range rsvs {
			if _, exist := matchedRsvMap[rsv]; exist {
				continue
			}
			cPkData, err = b.getPkDataFromSig(rsv, msgHash, true)
			if err == nil && bytes.Equal(cPkData, fromPubkeyData) {
				matchedRsvMap[rsv] = struct{}{}
				newRsvs = append(newRsvs, rsv)
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("msgHash %v hash no matched rsv", msgHash)
		}
	}
	return newRsvs, err
}
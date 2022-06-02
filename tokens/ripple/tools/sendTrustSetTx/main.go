package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/crypto"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
	"github.com/btcsuite/btcd/btcec"
)

var (
	bridge = ripple.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string
	paramPrivateKey string
	paramPublicKey  string
	paramSequence   string
	paraFee         string
	paramMemo       string
	paramFlags      uint64

	paramLimitAmount string
	paramQualityIn   string
	paramQualityOut  string

	qualityIn, qualityOut *uint32

	mpcConfig *mpc.Config

	chainID = big.NewInt(0)
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	ripplePubKey := ripple.ImportPublicKey(common.FromHex(paramPublicKey))
	pubkeyAddr := ripple.GetAddress(ripplePubKey, nil)

	log.Infof("signer address is %v", pubkeyAddr)

	var err error
	var sequence uint64

	if paramSequence != "" {
		sequence, err = common.GetUint64FromStr(paramSequence)
	} else {
		sequence, err = bridge.GetPoolNonce(pubkeyAddr, "pending")
	}
	if err != nil {
		log.Fatal("get account sequence failed", "err", err)
	}
	log.Info("get account sequence success", "sequence", sequence)

	rawTx, err := buildTrustSetTx(
		ripplePubKey, nil, uint32(sequence),
		paraFee, paramMemo, uint32(paramFlags),
		paramLimitAmount, qualityIn, qualityOut,
	)
	if err != nil {
		log.Fatal("build tx failed", "err", err)
	}

	signedTx, txHash, err := signTrustSetTx(rawTx, paramPublicKey)
	if err != nil {
		log.Fatal("sign tx failed", "err", err)
	}
	log.Info("sign tx success", "txHash", txHash)

	txHash, err = bridge.SendTransaction(signedTx)
	if err != nil {
		log.Fatal("send tx failed", "err", err)
	}
	log.Info("send tx success", "txHash", txHash)
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramPrivateKey, "priKey", "", "(optinal) signer private key")
	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")
	flag.StringVar(&paramSequence, "sequence", "", "(optional) signer sequence")
	flag.StringVar(&paraFee, "fee", "10", "(optional) fee amount")
	flag.StringVar(&paramMemo, "memo", "", "(optional) memo string")
	flag.Uint64Var(&paramFlags, "flags", 0, "(optional) tx flags")
	flag.StringVar(&paramLimitAmount, "limitAmount", "", "trust set LimitAmount")
	flag.StringVar(&paramQualityIn, "qualityIn", "", "(optional) trust set QualityIn")
	flag.StringVar(&paramQualityOut, "qualityOut", "", "(optional) trust set QualityOut")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

	if paramQualityIn != "" {
		qin, err := common.GetUint64FromStr(paramQualityIn)
		if err != nil {
			log.Fatal("wrong param qualityIn", "err", err)
		}
		vqin := uint32(qin)
		qualityIn = &vqin
	}

	if paramQualityOut != "" {
		qout, err := common.GetUint64FromStr(paramQualityOut)
		if err != nil {
			log.Fatal("wrong param qualityOut", "err", err)
		}
		vqout := uint32(qout)
		qualityOut = &vqout
	}
	log.Info("init flags finished")
}

func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	mpcConfig = mpc.InitConfig(config.MPC, true)
	log.Info("init config finished")
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

func buildTrustSetTx(
	key crypto.Key, keyseq *uint32, txseq uint32,
	fee, memo string, flags uint32,
	limitAmount string, qualityIn, qualityOut *uint32,
) (*data.TrustSet, error) {
	amount, err := data.NewAmount(limitAmount)
	if err != nil {
		return nil, err
	}
	tx := &data.TrustSet{
		LimitAmount: *amount,
		QualityIn:   qualityIn,
		QualityOut:  qualityOut,
	}
	tx.TransactionType = data.TRUST_SET

	txFlags := data.TransactionFlag(flags)
	tx.Flags = &txFlags

	if memo != "" {
		memoStr := new(data.Memo)
		memoStr.Memo.MemoData = []byte(memo)
		tx.Memos = append(tx.Memos, *memoStr)
	}

	base := tx.GetBase()

	base.Sequence = txseq

	fei, err := data.NewValue(fee, true)
	if err != nil {
		return nil, err
	}
	base.Fee = *fei

	copy(base.Account[:], key.Id(keyseq))

	tx.InitialiseForSigning()
	copy(tx.GetPublicKey().Bytes(), key.Public(keyseq))
	hash, msg, err := data.SigningHash(tx)
	if err != nil {
		return nil, err
	}
	log.Info("Build unsigned trustset success",
		"limitAmount", limitAmount, "qualityIn", qualityIn, "qualityOut", qualityOut,
		"memo", memo, "fee", fee, "sequence", txseq, "txflags", txFlags.String(),
		"signing hash", hash.String(), "blob", fmt.Sprintf("%X", msg))

	return tx, nil
}

func signTrustSetTx(tx *data.TrustSet, pubkeyStr string) (signedTx interface{}, txHash string, err error) {
	if paramPrivateKey != "" {
		return bridge.SignTransactionWithPrivateKey(tx, paramPrivateKey)
	}

	msgContext := "signTrustSetTx"
	msgHash, msg, err := data.SigningHash(tx)
	if err != nil {
		return nil, "", fmt.Errorf("get transaction signing hash failed: %w", err)
	}
	msg = append(tx.SigningPrefix().Bytes(), msg...)

	pubkey := common.FromHex(pubkeyStr)
	isEd := isEd25519Pubkey(pubkey)

	var keyID string
	var rsvs []string

	if isEd {
		// mpc ed public key has no 0xed prefix
		signPubKey := pubkeyStr[2:]
		// the real sign content is (signing prefix + msg)
		// when we hex encoding here, the mpc should do hex decoding there.
		signContent := common.ToHex(msg)
		keyID, rsvs, err = mpcConfig.DoSignOneED(signPubKey, signContent, msgContext)
	} else {
		signPubKey := pubkeyStr
		signContent := msgHash.String()
		keyID, rsvs, err = mpcConfig.DoSignOneEC(signPubKey, signContent, msgContext)
	}

	if err != nil {
		return nil, "", err
	}
	log.Info("MPCSignTransaction finished", "keyID", keyID)

	if len(rsvs) != 1 {
		return nil, "", fmt.Errorf("get sign status require one rsv but have %v (keyID = %v)", len(rsvs), keyID)
	}

	rsv := rsvs[0]
	log.Trace("MPCSignTransaction get rsv success", "keyID", keyID, "rsv", rsv)

	sig := rsvToSig(rsv, isEd)
	valid, err := crypto.Verify(pubkey, msgHash.Bytes(), msg, sig)
	if !valid || err != nil {
		return nil, "", fmt.Errorf("verify signature error (valid: %v): %v", valid, err)
	}

	signedTx, err = ripple.MakeSignedTransaction(pubkey, rsv, tx)
	if err != nil {
		return signedTx, "", err
	}

	txhash := signedTx.(data.Transaction).GetHash().String()

	return signedTx, txhash, nil
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

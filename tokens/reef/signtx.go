package reef

import (
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
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
)

func (b *Bridge) verifyTransactionReceiver(rawTx interface{}, tokenID string) (*ReefTransaction, error) {
	tx, ok := rawTx.(*ReefTransaction)
	if !ok {
		return nil, errors.New("[sign] wrong raw tx param")
	}
	if tx == nil || tx.To == nil {
		return nil, errors.New("[sign] tx receiver is empty")
	}
	checkReceiver, err := router.GetTokenRouterContract(tokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(*tx.To, checkReceiver) {
		return nil, fmt.Errorf("[sign] tx receiver mismatch. have %v want %v", tx.To, checkReceiver)
	}
	return tx, nil
}

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, err := b.verifyTransactionReceiver(rawTx, args.GetTokenID())
	if err != nil {
		return nil, "", err
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

	signer, err := b.PublicKeyToAddress(mpcPubkey)
	if err != nil {
		return nil, "", err
	}
	if strings.EqualFold(signer, args.From) {
		return nil, "", fmt.Errorf("signer dismatch from:%s, signer:%s", args.From, signer)
	}

	script_param := tx.buildScriptParam()

	// script_param := []interface{}{
	// 	args.Input.String(),
	// 	evmAddr.Hex(),
	// 	signer,
	// 	args.To,
	// 	strconv.FormatUint(*args.Extra.Gas, 10),
	// 	strconv.FormatUint(args.Extra.EthExtra.GasPrice.Uint64(), 10),
	// 	*args.Extra.BlockHash,
	// 	strconv.FormatUint(*args.Extra.Sequence, 10),
	// 	strconv.FormatUint(*args.Extra.EthExtra.Nonce, 10),
	// }
	msgHash, err := BuildSigningMessage(script_param)
	if err != nil {
		return nil, "", err
	}
	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid, "msghash", msgHash)
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, msgHash, msgContext)
	if err != nil {
		log.Info(logPrefix+"failed", "keyID", keyID, "txid", txid, "err", err)
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
	signature := common.FromHex(rsv)
	if len(signature) != crypto.SignatureLength {
		log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(signature), "want", crypto.SignatureLength)
		return nil, "", errors.New("wrong signature length")
	}

	tx.Signature = &rsv

	script_param = append(script_param, rsv)
	txHash, err = GetTxHash(script_param)
	if err != nil {
		return nil, "", err
	}
	tx.TxHash = &txHash
	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash, "nonce", tx.AccountNonce)
	return tx, txHash, nil
}

// GetSignedTxHashOfKeyID get signed tx hash by keyID (called by oracle)
func (b *Bridge) GetSignedTxHashOfKeyID(sender, keyID string, rawTx interface{}) (txHash string, err error) {
	tx, ok := rawTx.(*ReefTransaction)
	if !ok {
		return "", errors.New("wrong raw tx of keyID " + keyID)
	}
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	rsvs, err := mpcConfig.GetSignStatusByKeyID(keyID)
	if err != nil {
		return "", err
	}
	if len(rsvs) != 1 {
		return "", errors.New("wrong number of rsvs of keyID " + keyID)
	}

	rsv := rsvs[0]
	signature := common.FromHex(rsv)
	if len(signature) != crypto.SignatureLength {
		return "", errors.New("wrong signature of keyID " + keyID)
	}

	tx.Signature = &rsv

	script_param := tx.buildScriptParam()
	txHash, err = GetTxHash(script_param)
	if err != nil {
		return "", err
	}
	tx.TxHash = &txHash
	return txHash, nil
}

// SignTransactionWithPrivateKey sign tx with private key (use for testing)
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, priKey string) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*ReefTransaction)
	if !ok {
		return nil, "", errors.New("wrong raw tx param")
	}

	result, err := SignMessageWithPrivate(tx.buildScriptParam())
	if err != nil {
		return nil, "", err
	}

	tx.Signature = &result[0]
	tx.TxHash = &result[1]

	return tx, result[1], err
}

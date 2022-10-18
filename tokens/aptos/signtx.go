package aptos

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func (b *Bridge) verifyTransactionWithArgs(tx *Transaction, args *tokens.BuildTxArgs) error {
	swapin := tx.Payload.Arguments

	// receiver: address, amount: u64, _fromEvent: string, _fromChainID: u64

	fromChainID, err := strconv.ParseUint(swapin[3].(string), 10, 64)
	if err != nil || fromChainID != args.FromChainID.Uint64() {
		return fmt.Errorf("[sign] verify FromChainID failed")
	}

	// no need to check 2022/10/14
	// amount, err := strconv.ParseUint(swapin[1].(string), 10, 64)
	// if err != nil || amount != args.OriginValue.Uint64() {
	// 	return fmt.Errorf("[sign] verify Amount failed swapin.Amount %v args.OriginValue %v", amount, args.OriginValue.Uint64())
	// }

	if swapin[2] != args.SwapID {
		return fmt.Errorf("[sign] verify Tx failed swapin tx: %v OriginFrom: %v ", swapin[2], args.SwapID)
	}

	if swapin[0] != args.Bind {
		return fmt.Errorf("[sign] verify Tx failed swapin address: %v OriginAddress: %v ", swapin[0], args.Bind)
	}

	return nil
}

// MPCSignTransaction impl
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*Transaction)
	if !ok {
		return nil, "", errors.New("wrong signed transaction type")
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

	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return nil, "", tokens.ErrMissMPCPublicKey
	}

	signingMessage, err := b.Client.GetSigningMessage(tx)
	if err != nil {
		return nil, "", fmt.Errorf("unable to encode message for signing: %w", err)
	}
	msgContent := *signingMessage

	jsondata, err := json.Marshal(args.GetExtraArgs())
	if err != nil {
		return nil, "", fmt.Errorf("json marshal args failed: %w", err)
	}
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "msgContent", msgContent)
	log.Info(logPrefix+"start", "txid", txid, "fromChainID", args.FromChainID, "toChainID", args.ToChainID)

	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneED(mpcPubkey, msgContent, msgContext)
	if err != nil {
		return nil, "", err
	}
	log.Info(logPrefix+"finished", "keyID", keyID, "txid", txid, "fromChainID", args.FromChainID, "toChainID", args.ToChainID)

	if len(rsvs) != 1 {
		log.Error("get sign status require one rsv but return many",
			"rsvs", len(rsvs), "keyID", keyID, "txid", txid, "fromChainID", args.FromChainID, "toChainID", args.ToChainID)
		return nil, "", errors.New("get sign status require one rsv but return many")
	}

	rsv := rsvs[0]
	log.Trace(logPrefix+"get rsv signature success", "keyID", keyID, "txid", txid, "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "rsv", rsv)

	// Simulated transactions must have a non-valid signature
	_, err = b.Client.SimulateTranscation(tx, mpcPubkey)
	if err != nil {
		return nil, "", err
	}

	tx.Signature = &TransactionSignature{
		Type:      "ed25519_signature",
		PublicKey: mpcPubkey,
		Signature: rsv,
	}
	// only for swapin
	// receiver: address, amount: u64, _fromEvent: string, _fromChainID: u64
	txHash, err = b.CalcTxHashByTSScirpt(tx, "address,uint64,string,uint64")
	if err != nil {
		return nil, "", err
	}

	return tx, txHash, nil
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*Transaction)
	if !ok {
		return nil, "", errors.New("wrong signed transaction type")
	}
	account := NewAccountFromSeed(privKey)
	// Simulated transactions must have a non-valid signature
	_, err = b.Client.SimulateTranscation(tx, account.GetPublicKeyHex())
	if err != nil {
		return nil, "", err
	}
	signingMessage, err := b.Client.GetSigningMessage(tx)
	if err != nil {
		log.Fatal("GetSigningMessage", "err", err)
	}
	signature, err := account.SignString(*signingMessage)
	if err != nil {
		log.Fatal("SignString", "err", err)
	}
	tx.Signature = &TransactionSignature{
		Type:      "ed25519_signature",
		PublicKey: account.GetPublicKeyHex(),
		Signature: signature,
	}
	log.Info("SignTransactionWithPrivateKey", "signature", signature)
	// only for swapin
	// receiver: address, amount: u64, _fromEvent: string, _fromChainID: u64
	txHash, err = b.CalcTxHashByTSScirpt(tx, "address,uint64,string,uint64")
	if err != nil {
		return nil, "", err
	}
	return tx, txHash, err
}

func (b *Bridge) CalcTxHashByTSScirpt(rawTx interface{}, argTypes string) (txHash string, err error) {
	tx, ok := rawTx.(*Transaction)
	if !ok {
		return "", fmt.Errorf("not aptos Transaction")
	}

	jsonStr, err := json.Marshal(tx)
	if err != nil {
		return "", err
	}

	ledgerInfo, err := b.Client.GetLedger()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("yarn txhash '%s' %s %d", string(jsonStr), argTypes, ledgerInfo.ChainId))
	out, err := cmd.CombinedOutput()
	stats := strings.Split(string(out), "\n")
	for i, stat := range stats {
		log.Info("CalcTxHashByTSScirpt", strconv.Itoa(i), stat)
	}
	if err != nil {
		return "", fmt.Errorf(string(out))
	}
	if len(stats) < 3 {
		return "", fmt.Errorf("CalcTxHashByTSScirpt ts output error")
	}
	if !strings.HasPrefix(stats[len(stats)-2], "Done") {
		return "", fmt.Errorf(stats[len(stats)-2])
	}
	return stats[len(stats)-3], nil

}

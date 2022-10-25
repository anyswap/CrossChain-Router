package aptos

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// MPCSignTransaction impl
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*Transaction)
	if !ok {
		return nil, "", errors.New("wrong signed transaction type")
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

	signingMessage, err := b.GetSigningMessage(tx)
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
	err = b.SimulateTranscation(tx, account.GetPublicKeyHex())
	if err != nil {
		return nil, "", err
	}
	signingMessage, err := b.GetSigningMessage(tx)
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
	// txHash, err = b.CalcTxHashByTSScirpt(tx, "address,uint64,string,uint64")
	txHash, err = b.CalcTxHashByTSScirpt(tx, "address,uint64")
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

	ledgerInfo, err := b.GetLedger()
	if err != nil {
		return "", err
	}

	txbody := string(jsonStr)
	return RunTxHashScript(&txbody, &argTypes, ledgerInfo.ChainId)
}

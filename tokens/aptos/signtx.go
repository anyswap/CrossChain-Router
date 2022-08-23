package aptos

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

func (b *Bridge) verifyTransactionWithArgs(tx *Transaction, args *tokens.BuildTxArgs) error {
	fmt.Println(tx.Payload)

	swapin := tx.Payload.Arguments

	// receiver: address, amount: u64, _fromEvent: vector<u8>, _fromChainID: u64

	fromChainID, err := strconv.ParseUint(swapin[3], 10, 64)
	if err != nil || fromChainID != args.FromChainID.Uint64() {
		return fmt.Errorf("[sign] verify FromChainID failed")
	}

	amount, err := strconv.ParseUint(swapin[1], 10, 64)
	if err != nil || amount != args.OriginValue.Uint64() {
		return fmt.Errorf("[sign] verify Amount failed swapin.Amount %v args.OriginValue %v", amount, args.OriginValue.Uint64())
	}

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

	// sig, err := types.NewSignatureFromString(rsv)
	// if err != nil {
	// 	log.Error("get signature from rsv failed", "keyID", keyID, "txid", txid, "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "err", err)
	// 	return nil, "", err
	// }

	tx.Signature = &TransactionSignature{
		Type:      "ed25519_signature",
		PublicKey: mpcPubkey,
		Signature: rsv,
	}

	txInfo, err := b.Client.SubmitTranscation(tx)
	if err != nil {
		return nil, "", err
	}

	return tx, txInfo.Hash, nil
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*Transaction)
	if !ok {
		return nil, "", errors.New("wrong signed transaction type")
	}
	account := NewAccountFromSeed(privKey)
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

	txInfo, err := b.Client.SubmitTranscation(tx)
	if err != nil {
		return nil, "", err
	}

	return tx, txInfo.Hash, err
}

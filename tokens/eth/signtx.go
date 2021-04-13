package eth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/mpc"
	"github.com/anyswap/CrossChain-Router/tokens"
	"github.com/anyswap/CrossChain-Router/tools/crypto"
	"github.com/anyswap/CrossChain-Router/types"
)

func (b *Bridge) verifyTransactionReceiver(rawTx interface{}) (*types.Transaction, error) {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return nil, errors.New("[sign] wrong raw tx param")
	}
	if tx.To() == nil || *tx.To() == (common.Address{}) {
		return nil, errors.New("[sign] tx receiver is empty")
	}
	checkReceiver := b.ChainConfig.RouterContract
	if !strings.EqualFold(tx.To().String(), checkReceiver) {
		return nil, fmt.Errorf("[sign] tx receiver mismatch. have %v want %v", tx.To().String(), checkReceiver)
	}
	return tx, nil
}

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, err := b.verifyTransactionReceiver(rawTx)
	if err != nil {
		return nil, "", err
	}

	gasPrice, err := b.getGasPrice()
	if err == nil && args.Extra.EthExtra.GasPrice.Cmp(gasPrice) < 0 {
		args.Extra.EthExtra.GasPrice = gasPrice
	}

	mpcAddress := b.ChainConfig.GetRouterMPC()
	mpcPubkey := b.ChainConfig.GetRouterMPCPubkey()

	signer := b.Signer
	msgHash := signer.Hash(tx)
	jsondata, _ := json.Marshal(args)
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid, "msghash", msgHash.String())
	keyID, rsvs, err := mpc.DoSignOne(mpcPubkey, msgHash.String(), msgContext)
	if err != nil {
		return nil, "", err
	}
	log.Info(logPrefix+"finished", "keyID", keyID, "txid", txid, "msghash", msgHash.String())

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

	signedTx, err := tx.WithSignature(signer, signature)
	if err != nil {
		return nil, "", err
	}

	sender, err := types.Sender(signer, signedTx)
	if err != nil {
		return nil, "", err
	}

	if !strings.EqualFold(sender.String(), mpcAddress) {
		log.Error(logPrefix+"verify sender failed", "have", sender.String(), "want", mpcAddress)
		return nil, "", errors.New("wrong sender address")
	}
	txHash = signedTx.Hash().String()
	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash, "nonce", signedTx.Nonce())
	return signedTx, txHash, err
}

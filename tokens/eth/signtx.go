package eth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/anyswap/CrossChain-Router/v3/types"
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

	if !params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID) {
		gasPrice, errt := b.getGasPrice(args)
		if errt == nil && args.Extra.EthExtra.GasPrice.Cmp(gasPrice) < 0 {
			log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction update gas price", "txid", args.SwapID, "oldGasPrice", args.Extra.EthExtra.GasPrice, "newGasPrice", gasPrice)
			args.Extra.EthExtra.GasPrice = gasPrice
			tx.SetGasPrice(gasPrice)
		}
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

	signedTx, err := b.signTxWithSignature(tx, signature, common.HexToAddress(mpcAddress))
	if err != nil {
		return nil, "", err
	}
	txHash = signedTx.Hash().String()
	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash, "nonce", signedTx.Nonce())
	return signedTx, txHash, nil
}

func (b *Bridge) signTxWithSignature(tx *types.Transaction, signature []byte, signerAddr common.Address) (*types.Transaction, error) {
	signer := b.Signer
	vPos := crypto.SignatureLength - 1
	for i := 0; i < 2; i++ {
		signedTx, err := tx.WithSignature(signer, signature)
		if err != nil {
			return nil, err
		}

		sender, err := types.Sender(signer, signedTx)
		if err != nil {
			return nil, err
		}

		if sender == signerAddr {
			return signedTx, nil
		}

		signature[vPos] ^= 0x1 // v can only be 0 or 1
	}

	return nil, errors.New("wrong sender address")
}

// GetSignedTxHashOfKeyID get signed tx hash by keyID (called by oracle)
func (b *Bridge) GetSignedTxHashOfKeyID(keyID string, rawTx interface{}) (txHash string, err error) {
	tx, ok := rawTx.(*types.Transaction)
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
	signature := common.FromHex(rsv)
	if len(signature) != crypto.SignatureLength {
		return "", errors.New("wrong signature of keyID " + keyID)
	}

	mpcAddress := b.ChainConfig.GetRouterMPC()
	signedTx, err := b.signTxWithSignature(tx, signature, common.HexToAddress(mpcAddress))
	if err != nil {
		return "", err
	}

	txHash = signedTx.Hash().String()
	return txHash, nil
}

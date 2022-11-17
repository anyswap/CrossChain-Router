package tron

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	tronaddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	//nolint:staticcheck // ignore SA1019
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/protobuf/proto"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
)

func getTriggerSmartContract(tx *core.Transaction) (*core.TriggerSmartContract, error) {
	rawdata := tx.GetRawData()
	contracts := rawdata.GetContract()

	var contract core.TriggerSmartContract
	//nolint:staticcheck // ignore SA1019
	err := ptypes.UnmarshalAny(contracts[0].GetParameter(), &contract)
	if err != nil {
		return nil, fmt.Errorf("decode trigger smart contract tx error: %w", err)
	}
	return &contract, nil
}

func (b *Bridge) verifyTransactionReceiver(rawTx interface{}, tokenID string) (*core.Transaction, error) {
	tx, ok := rawTx.(*core.Transaction)
	if !ok {
		return nil, errors.New("wrong raw tx param")
	}

	contract, err := getTriggerSmartContract(tx)
	if err != nil {
		return nil, err
	}

	txRecipient := tronaddress.Address(contract.ContractAddress).String()

	checkReceiver, err := router.GetTokenRouterContract(tokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(txRecipient, checkReceiver) {
		return nil, fmt.Errorf("[sign] tx receiver mismatch. have %v want %v", txRecipient, checkReceiver)
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

	txHash = CalcTxHash(tx)
	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid, "msghash", txHash)
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, txHash, msgContext)
	if err != nil {
		return nil, "", err
	}
	log.Info(logPrefix+"finished", "keyID", keyID, "txid", txid, "msghash", txHash)

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

	tx.Signature = append(tx.Signature, signature)
	signedTx := tx
	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash)
	return signedTx, txHash, nil
}

// GetSignedTxHashOfKeyID get signed tx hash by keyID (called by oracle)
func (b *Bridge) GetSignedTxHashOfKeyID(sender, keyID string, rawTx interface{}) (txHash string, err error) {
	tx, ok := rawTx.(*core.Transaction)
	if !ok {
		return "", errors.New("wrong raw tx param")
	}
	txhash := CalcTxHash(tx)
	return txhash, nil
}

// SignTransactionWithPrivateKey sign tx with private key (use for testing)
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, priKey string) (signTx interface{}, txHash string, err error) {
	privKey, err := crypto.ToECDSA(common.FromHex(priKey))
	if err != nil {
		return nil, "", err
	}

	// rawTx is of type authtypes.StdSignDoc
	tx, ok := rawTx.(*core.Transaction)
	if !ok {
		return nil, "", errors.New("wrong raw tx param")
	}

	rawData, err := proto.Marshal(tx.GetRawData())
	if err != nil {
		return nil, "", err
	}
	h256h := sha256.New()
	_, err = h256h.Write(rawData)
	if err != nil {
		return nil, "", err
	}
	hash := h256h.Sum(nil)
	txhash := fmt.Sprintf("%X", hash)

	signature, err := crypto.Sign(hash, privKey)
	if err != nil {
		return nil, "", err
	}
	tx.Signature = append(tx.Signature, signature)
	return tx, txhash, nil
}

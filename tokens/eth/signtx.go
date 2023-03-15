package eth

import (
	"encoding/hex"
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
	"github.com/anyswap/CrossChain-Router/v3/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/zksync-sdk/zksync2-go"
)

func (b *Bridge) verifyTransactionReceiver(rawTx interface{}, tokenID string) (*types.Transaction, error) {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return nil, errors.New("[sign] wrong raw tx param")
	}
	if tx.To() == nil || *tx.To() == (common.Address{}) {
		return nil, errors.New("[sign] tx receiver is empty")
	}
	checkReceiver, err := router.GetTokenRouterContract(tokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(tx.To().String(), checkReceiver) {
		return nil, fmt.Errorf("[sign] tx receiver mismatch. have %v want %v", tx.To().String(), checkReceiver)
	}
	return tx, nil
}

func (b *Bridge) verifyZkSyncTransactionReceiver(rawTx interface{}, tokenID string) (*zksync2.Transaction712, error) {
	tx, ok := rawTx.(*zksync2.Transaction712)
	if !ok {
		return nil, errors.New("[sign] wrong raw tx param")
	}
	if tx.To == nil || *tx.To == (ethcommon.Address{}) {
		return nil, errors.New("[sign] tx receiver is empty")
	}
	checkReceiver, err := router.GetTokenRouterContract(tokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(tx.To.String(), checkReceiver) {
		return nil, fmt.Errorf("[sign] tx receiver mismatch. have %v want %v", tx.To.String(), checkReceiver)
	}
	return tx, nil
}

func HashTypedData(data apitypes.TypedData) ([]byte, error) {
	domain, err := data.HashStruct("EIP712Domain", data.Domain.Map())
	if err != nil {
		return nil, fmt.Errorf("failed to get hash of typed data domain: %w", err)
	}
	dataHash, err := data.HashStruct(data.PrimaryType, data.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to get hash of typed message: %w", err)
	}
	prefixedData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domain), string(dataHash)))
	prefixedDataHash := crypto.Keccak256(prefixedData)
	return prefixedDataHash, nil
}

func (b *Bridge) MPCSignZkSyncTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, err := b.verifyZkSyncTransactionReceiver(rawTx, args.GetTokenID())
	if err != nil {
		return nil, "", err
	}

	chainID, _ := b.ChainID()
	domain := zksync2.DefaultEip712Domain(chainID.Int64())
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			tx.GetEIP712Type():     tx.GetEIP712Types(),
			domain.GetEIP712Type(): domain.GetEIP712Types(),
		},
		PrimaryType: tx.GetEIP712Type(),
		Domain:      domain.GetEIP712Domain(),
		Message:     tx.GetEIP712Message(),
	}
	msgHash, err := HashTypedData(typedData)
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

	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid, "msghash", fmt.Sprintf("%x", msgHash))
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, fmt.Sprintf("%x", msgHash), msgContext)
	if err != nil {
		log.Info(logPrefix+"failed", "keyID", keyID, "txid", txid, "err", err)
		return nil, "", err
	}
	log.Info(logPrefix+"finished", "keyID", keyID, "txid", txid, "msghash", fmt.Sprintf("%x", msgHash))

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

	sig, _ := hex.DecodeString(rsv)
	if sig[64] < 27 {
		sig[64] += 27
	}

	signedTx, err := tx.RLPValues(sig)
	if err != nil {
		return nil, "", err
	}

	digest := []byte{}
	digest = append(digest, msgHash...)
	digest = append(digest, crypto.Keccak256(sig)...)
	txHash = fmt.Sprintf("0x%x", crypto.Keccak256(digest))

	log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash, "nonce", tx.Nonce)
	return signedTx, txHash, nil
}

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	if b.IsZKSync() {
		return b.MPCSignZkSyncTransaction(rawTx, args)
	}
	tx, err := b.verifyTransactionReceiver(rawTx, args.GetTokenID())
	if err != nil {
		return nil, "", err
	}

	if !params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID) {
		gasPrice, errt := b.getGasPrice(args)
		if errt == nil && args.Extra.GasPrice.Cmp(gasPrice) < 0 {
			log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction update gas price", "txid", args.SwapID, "oldGasPrice", args.Extra.GasPrice, "newGasPrice", gasPrice)
			args.Extra.GasPrice = gasPrice
			tx.SetGasPrice(gasPrice)
		}
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

	signer := b.Signer
	msgHash := signer.Hash(tx)
	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "txid", txid, "msghash", msgHash.String())
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, msgHash.String(), msgContext)
	if err != nil {
		log.Info(logPrefix+"failed", "keyID", keyID, "txid", txid, "err", err)
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

	signedTx, err := b.signTxWithSignature(tx, signature, common.HexToAddress(args.From))
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
func (b *Bridge) GetSignedTxHashOfKeyID(sender, keyID string, rawTx interface{}) (txHash string, err error) {
	tx, ok := rawTx.(*types.Transaction)
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

	signedTx, err := b.signTxWithSignature(tx, signature, common.HexToAddress(sender))
	if err != nil {
		return "", err
	}

	txHash = signedTx.Hash().String()
	return txHash, nil
}

// SignTransactionWithPrivateKey sign tx with private key (use for testing)
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, priKey string) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return nil, "", errors.New("wrong raw tx param")
	}

	privKey, err := crypto.ToECDSA(common.FromHex(priKey))
	if err != nil {
		return nil, "", err
	}

	signedTx, err := types.SignTx(tx, b.Signer, privKey)
	if err != nil {
		return nil, "", fmt.Errorf("sign tx failed, %w", err)
	}

	txHash = signedTx.Hash().String()
	log.Info(b.ChainConfig.BlockChain+" SignTransaction success", "txhash", txHash, "nonce", signedTx.Nonce())
	return signedTx, txHash, err
}

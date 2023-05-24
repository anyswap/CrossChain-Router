package starknet

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet/rpcv02"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
	"github.com/dontpanicdao/caigo/types"
)

// SignTransactionWithPrivateKey sign tx with StarkCurve private key
// StarkCurve signature is not compatible with Secp256k1 signature, depending on account contract's
// encryption/validation scheme,
func (b *Bridge) SignTransactionWithPrivateKey(curveType string, rawTx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	if curveType == EC256STARK && privKey == "" && b.account.private == "" {
		return nil, "", fmt.Errorf("empty private key")
	}
	tx, ok := rawTx.(FunctionCallWithDetails)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}
	hash, err := b.TransactionHash(tx.Call, tx.Nonce, tx.MaxFee)
	if err != nil {
		return nil, "", err
	}

	var r, s, v *big.Int
	var signature []string

	switch curveType {
	case EC256K1:
		sk, err := crypto.ToECDSA(common.FromHex(privKey))
		if err != nil {
			return nil, "", err
		}
		h := types.BigToHash(hash)
		sig, err := crypto.Sign(h[:], sk)
		if err != nil {
			return nil, "", err
		}
		r, s, v = DecodeSignature(sig)
		signature = ConvertSignature(r, s, v)
	case EC256STARK:
		if b.account.private == "" {
			b.account.private = privKey
		}
		r, s, err = b.account.Sign(hash)
		if err != nil {
			return nil, "", err
		}
		signature = append(signature, fmt.Sprintf("0x%s", r.Text(16)))
		signature = append(signature, fmt.Sprintf("0x%s", s.Text(16)))
	default:
		return nil, "", fmt.Errorf("unsupported curve in Starknet: %s", curveType)
	}

	signedTx = rpcv02.BroadcastedInvokeV1Transaction{
		MaxFee:        tx.MaxFee,
		Version:       rpcv02.TransactionV1,
		Signature:     signature,
		Nonce:         tx.Nonce,
		Type:          TxTypeInvoke,
		Calldata:      tx.Call.Calldata,
		SenderAddress: types.HexToHash(b.account.Address),
	}
	return signedTx, txHash, nil
}

//func (b *Bridge) Sign(args *tokens.BuildTxArgs, mpcPubkey string, hash string, msgContext string) {
//
//	var keyID string
//	var rsvs []string
//
//	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
//
//	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, hash, msgContext)
//
//	if err != nil {
//		log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction failed", "keyID", keyID, "txid", args.SwapID, "err", err)
//		return nil, "", err
//	}
//	log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction finished", "keyID", keyID, "txid", args.SwapID)
//
//	if len(rsvs) != 1 {
//		return nil, "", fmt.Errorf("get sign status require one rsv but have %v (keyID = %v)", len(rsvs), keyID)
//	}
//
//	rsv := rsvs[0]
//	log.Trace(b.ChainConfig.BlockChain+" MPCSignTransaction get rsv success", "keyID", keyID, "rsv", rsv)
//
//	sig := common.FromHex(rsv)
//	r, s, v := DecodeSignature(sig)
//
//}

func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(FunctionCallWithDetails)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}
	h, err := b.TransactionHash(tx.Call, tx.Nonce, tx.MaxFee)
	if err != nil {
		return nil, "", err
	}

	txHash = h.String()

	// MPC sign
	mpcParams := params.GetMPCConfig(b.UseFastMPC)
	if mpcParams.SignWithPrivateKey {
		priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
		return b.SignTransactionWithPrivateKey(EC256STARK, rawTx, priKey)
	}

	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return nil, "", tokens.ErrMissMPCPublicKey
	}

	var keyID string
	var rsvs []string

	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)

	signPubKey := mpcPubkey

	keyID, rsvs, err = mpcConfig.DoSignOneEC(signPubKey, h.String(), msgContext)

	if err != nil {
		log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction failed", "keyID", keyID, "txid", args.SwapID, "err", err)
		return nil, "", err
	}
	log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction finished", "keyID", keyID, "txid", args.SwapID)

	if len(rsvs) != 1 {
		return nil, "", fmt.Errorf("get sign status require one rsv but have %v (keyID = %v)", len(rsvs), keyID)
	}

	rsv := rsvs[0]
	log.Trace(b.ChainConfig.BlockChain+" MPCSignTransaction get rsv success", "keyID", keyID, "rsv", rsv)

	sig := common.FromHex(rsv)
	r, s, v := DecodeSignature(sig)

	signedTx = rpcv02.BroadcastedInvokeV1Transaction{
		MaxFee:        tx.MaxFee,
		Version:       rpcv02.TransactionV1,
		Signature:     ConvertSignature(r, s, v),
		Nonce:         tx.Nonce,
		Type:          TxTypeInvoke,
		Calldata:      tx.Call.Calldata,
		SenderAddress: types.HexToHash(b.account.Address),
	}
	return signedTx, txHash, nil
}

func (b *Bridge) MPCSign(signPubKey string, msgHash string, msgContext string) (string, []string, error) {
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	return mpcConfig.DoSignOneEC(signPubKey, msgHash, msgContext)
}

func DecodeSignature(sig []byte) (r, s, v *big.Int) {
	if len(sig) != crypto.SignatureLength {
		log.Warn(fmt.Sprintf("wrong size for signature: got %d, want %d", len(sig), crypto.SignatureLength))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64]}) // CC: Unlike Ethereum, SN uses raw v value
	return r, s, v
}

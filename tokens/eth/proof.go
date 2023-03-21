package eth

import (
	"encoding/json"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
)

var (
	errDontSupportProof = errors.New("don't support proof")
)

func (b *Bridge) CalcProofID(args *tokens.BuildTxArgs) (string, error) {
	anycallSwapInfo := args.AnyCallSwapInfo
	if anycallSwapInfo == nil {
		return "", errDontSupportProof
	}
	nonce, err := common.GetBigIntFromStr(anycallSwapInfo.Nonce)
	if err != nil {
		return "", err
	}
	flags, err := common.GetBigIntFromStr(anycallSwapInfo.Flags)
	if err != nil {
		return "", err
	}
	input := abicoder.PackData(
		common.HexToAddress(anycallSwapInfo.CallTo),
		anycallSwapInfo.CallData,
		anycallSwapInfo.ExtData,
		args.LogIndex,
		common.HexToAddress(anycallSwapInfo.CallFrom),
		args.FromChainID,
		common.HexToHash(args.SwapID),
		nonce,
		flags,
	)
	proofID := common.Keccak256Hash(input)
	return proofID.Hex(), nil
}

func (b *Bridge) GenerateProof(proofID string, args *tokens.BuildTxArgs) (string, error) {
	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return "", tokens.ErrMissMPCPublicKey
	}

	jsondata, _ := json.Marshal(args.GetExtraArgs())
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " GenerateProof "
	log.Info(logPrefix+"start", "txid", txid, "proofID", proofID)
	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneEC(mpcPubkey, proofID, msgContext)
	if err != nil {
		log.Info(logPrefix+"failed", "keyID", keyID, "txid", txid, "err", err)
		return "", err
	}
	log.Info(logPrefix+"finished", "keyID", keyID, "txid", txid, "proofID", proofID)

	if len(rsvs) != 1 {
		log.Warn("get sign status require one rsv but return many",
			"rsvs", len(rsvs), "keyID", keyID, "txid", txid)
		return "", errors.New("get sign status require one rsv but return many")
	}

	rsv := rsvs[0]
	log.Trace(logPrefix+"get rsv signature success", "keyID", keyID, "txid", txid, "rsv", rsv)
	signature := common.FromHex(rsv)
	if len(signature) != crypto.SignatureLength {
		log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(signature), "want", crypto.SignatureLength)
		return "", errors.New("wrong signature length")
	}

	recoveredPub, err := crypto.Ecrecover(common.FromHex(proofID), signature)
	if err != nil {
		return "", err
	}
	pubKey, err := crypto.UnmarshalPubkey(recoveredPub)
	if err != nil {
		return "", err
	}
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	if recoveredAddr != common.HexToAddress(args.From) {
		return "", errors.New("verify signature failed")
	}

	return rsv, nil
}

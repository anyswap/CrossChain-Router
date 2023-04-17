package eth

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
	"github.com/anyswap/CrossChain-Router/v3/tools/crypto"
)

func (b *Bridge) verifyProofID(rawTx interface{}, msgHashes []string) error {
	proofID, ok := rawTx.(string)
	if !ok {
		return tokens.ErrWrongProofID
	}
	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	msgHash := msgHashes[0]
	if proofID != msgHash {
		log.Trace("proofID mismatch", "want", msgHash, "have", proofID)
		return tokens.ErrProofIDMismatch
	}
	return nil
}

func (b *Bridge) CalcProofID(args *tokens.BuildTxArgs) (string, error) {
	anycallSwapInfo := args.AnyCallSwapInfo
	if anycallSwapInfo == nil {
		return "", tokens.ErrDontSupportProof
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
		anycallSwapInfo.AppID,
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
	mpcParams := params.GetMPCConfig(b.UseFastMPC)
	if mpcParams.SignWithPrivateKey {
		priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
		return b.GenerateProofWithPrivateKey(proofID, priKey)
	}

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

	signature[crypto.SignatureLength-1] += 27

	log.Info(logPrefix+"success", "txid", args.SwapID, "logIndex", args.LogIndex, "proofID", proofID)
	return common.ToHex(signature), nil
}

func (b *Bridge) GenerateProofWithPrivateKey(proofID, priKey string) (string, error) {
	signContent := common.FromHex(proofID)

	privKey, err := crypto.ToECDSA(common.FromHex(priKey))
	if err != nil {
		return "", err
	}

	signature, err := crypto.Sign(signContent[:], privKey)
	if err != nil {
		return "", err
	}

	signature[crypto.SignatureLength-1] += 27

	log.Info(b.ChainConfig.BlockChain+" GenerateProof success", "proofID", proofID)
	return common.ToHex(signature), nil
}

func (b *Bridge) SubmitProof(proofID, proof string, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, "", tokens.ErrToChainIDMismatch
	}

	routerContract := b.GetRouterContract("")
	consumed, err := b.IsProofConsumed(routerContract, proofID)
	if err != nil {
		return nil, "", err
	}
	if consumed {
		return nil, "", tokens.ErrProofConsumed
	}

	switch args.SwapType {
	case tokens.AnyCallSwapType:
		err = b.buildAnyCallWithProofTxInput(proofID, proof, args)
	default:
		return nil, "", tokens.ErrDontSupportProof
	}

	if err != nil {
		return nil, "", err
	}

	submitter := pickSubmitter(args.SignerIndex)
	args.From = submitter.Address
	args.SwapValue = big.NewInt(0)

	err = b.setDefaults(args)
	if err != nil {
		return nil, "", err
	}

	rawTx, err := b.buildTx(args)
	if err != nil {
		return nil, "", err
	}

	return b.SignTransactionWithPrivateKey(rawTx, submitter.GetPrivateKey())
}

func pickSubmitter(index int) *params.KeystoreConfig {
	submitters := params.GetProofSubmitters()
	submitter := submitters[index]
	return submitter
}

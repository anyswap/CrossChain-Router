package solana

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	routerprog "github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
	bin "github.com/streamingfast/binary"
)

func (b *Bridge) verifyTransactionWithArgs(tx *types.Transaction, args *tokens.BuildTxArgs) error {
	fmt.Println(tx.Message.Instructions[0].Data)

	var inst routerprog.Instruction
	if err := inst.UnmarshalBinary(bin.NewDecoder(tx.Message.Instructions[0].Data)); err != nil {
		return fmt.Errorf("unable to decode instruction: %w", err)
	}
	params, ok := inst.Impl.(routerprog.ISwapinParams)
	if !ok {
		return fmt.Errorf("unable to decode ISwapinParams")
	}
	swapin := params.GetSwapinParams()

	if swapin.FromChainID != args.FromChainID.Uint64() {
		return fmt.Errorf("[sign] verify FromChainID failed")
	}

	if swapin.Amount != args.OriginValue.Uint64() {
		return fmt.Errorf("[sign] verify Amount failed swapin.Amount %v args.OriginValue %v", swapin.Amount, args.OriginValue.Uint64())
	}

	if swapin.Tx.String() != args.SwapID {
		return fmt.Errorf("[sign] verify Tx failed swapin tx: %v OriginFrom: %v ", swapin.Tx.String(), args.SwapID)
	}

	return nil
}

// MPCSignTransaction impl
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return nil, "", errors.New("wrong signed transaction type")
	}

	err = b.verifyTransactionWithArgs(tx, args)
	if err != nil {
		log.Warn("Verify transaction failed", "error", err)
		return nil, "", err
	}

	signerKeys := tx.Message.SignerKeys()
	if len(signerKeys) != 1 {
		return nil, "", fmt.Errorf("wrong number of signer keys: %d", len(signerKeys))
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

	msgContent, err := tx.Message.Serialize()
	if err != nil {
		return nil, "", fmt.Errorf("unable to encode message for signing: %w", err)
	}

	jsondata, err := json.Marshal(args.GetExtraArgs())
	if err != nil {
		return nil, "", fmt.Errorf("json marshal args failed: %w", err)
	}
	msgContext := string(jsondata)

	txid := args.SwapID
	logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
	log.Info(logPrefix+"start", "msgContent", common.ToHex(msgContent))
	log.Info(logPrefix+"start", "txid", txid, "fromChainID", args.FromChainID, "toChainID", args.ToChainID)

	mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
	keyID, rsvs, err := mpcConfig.DoSignOneED(mpcPubkey, common.ToHex(msgContent), msgContext)
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

	sig, err := types.NewSignatureFromString(rsv)
	if err != nil {
		log.Error("get signature from rsv failed", "keyID", keyID, "txid", txid, "fromChainID", args.FromChainID, "toChainID", args.ToChainID, "err", err)
		return nil, "", err
	}

	tx.Signatures = append(tx.Signatures, sig)

	return tx, sig.String(), nil
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return nil, "", errors.New("wrong signed transaction type")
	}
	msgContent, err := tx.Message.Serialize()
	if err != nil {
		return nil, "", fmt.Errorf("unable to encode message for signing: %w", err)
	}
	signAccount, err := types.AccountFromPrivateKeyBase58(privKey)
	if err != nil {
		return nil, "", fmt.Errorf("unable to encode message for signing: %w", err)
	}
	signature, err := signAccount.PrivateKey.Sign(msgContent)

	tx.Signatures = append(tx.Signatures, signature)
	log.Info("SignTransactionWithPrivateKey", "signature", signature.String())
	return tx, signature.String(), err
}

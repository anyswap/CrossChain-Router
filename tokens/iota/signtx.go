package iota

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/iotaledger/hive.go/serializer"
	iotago "github.com/iotaledger/iota.go/v2"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	if messageBuilder, ok := rawTx.(*MessageBuilder); !ok {
		return nil, "", tokens.ErrWrongRawTx
	} else {
		mpcParams := params.GetMPCConfig(b.UseFastMPC)
		if mpcParams.SignWithPrivateKey {
			priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
			return b.SignTransactionWithPrivateKey(rawTx, priKey)
		}

		mpcPubkey := router.GetMPCPublicKey(args.From)
		if mpcPubkey == "" {
			return nil, "", tokens.ErrMissMPCPublicKey
		}

		mpcPubkeyByte, err := hex.DecodeString(mpcPubkey)
		if err != nil {
			return nil, "", err
		}
		if signMessage, err := messageBuilder.Essence.SigningMessage(); err != nil {
			return nil, "", err
		} else {
			jsondata, _ := json.Marshal(args.GetExtraArgs())
			msgContext := string(jsondata)

			txid := args.SwapID
			logPrefix := b.ChainConfig.BlockChain + " MPCSignTransaction "
			log.Info(logPrefix+"start", "txid", txid)

			mpcConfig := mpc.GetMPCConfig(b.UseFastMPC)
			keyID, rsvs, err := mpcConfig.DoSignOneED(mpcPubkey, common.ToHex(signMessage[:]), msgContext)
			if err != nil {
				return nil, "", err
			}

			if len(rsvs) != 1 {
				log.Warn("get sign status require one rsv but return many",
					"rsvs", len(rsvs), "keyID", keyID, "txid", txid)
				return nil, "", errors.New("get sign status require one rsv but return many")
			}

			rsv := rsvs[0]
			log.Trace(logPrefix+"get rsv signature success", "keyID", keyID, "txid", txid, "rsv", rsv)
			sig := common.FromHex(rsv)
			if len(sig) != ed25519.SignatureSize {
				log.Error("wrong signature length", "keyID", keyID, "txid", txid, "have", len(sig), "want", ed25519.SignatureSize)
				return nil, "", errors.New("wrong signature length")
			}

			signature := &iotago.Ed25519Signature{}
			copy(signature.Signature[:], sig)
			copy(signature.PublicKey[:], mpcPubkeyByte[:])

			unlockBlocks := serializer.Serializables{}
			for i := 0; i < len(messageBuilder.Essence.Inputs); i++ {
				switch i {
				case 0:
					unlockBlocks = append(unlockBlocks, &iotago.SignatureUnlockBlock{Signature: signature})
				default:
					unlockBlocks = append(unlockBlocks, &iotago.ReferenceUnlockBlock{Reference: uint16(0)})
				}
			}

			sigTxPayload := &iotago.Transaction{Essence: messageBuilder.Essence, UnlockBlocks: unlockBlocks}

			if message, err := b.ProofOfWork(iotago.NewMessageBuilder().Payload(sigTxPayload)); err != nil {
				return nil, "", err
			} else {
				log.Info(logPrefix+"success", "keyID", keyID, "txid", txid, "txhash", txHash)
				return message, iotago.MessageIDToHexString(message.MustID()), nil
			}
		}
	}

}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signTx interface{}, txHash string, err error) {
	mpc := b.GetRouterContract("")
	if edAddr := ConvertStringToAddress(mpc); edAddr != nil {
		priv, _ := hex.DecodeString(privKey)
		signKey := iotago.NewAddressKeysForEd25519Address(edAddr, priv)
		signer := iotago.NewInMemoryAddressSigner(signKey)

		tx := rawTx.(*MessageBuilder)
		if message, err := b.ProofOfWork(tx.TransactionBuilder.BuildAndSwapToMessageBuilder(signer, nil)); err == nil {
			return message, iotago.MessageIDToHexString(message.MustID()), nil
		} else {
			return nil, "", err
		}
	}
	return nil, "", tokens.ErrCommitMessage
}

func (b *Bridge) ProofOfWork(messageBuilder *iotago.MessageBuilder) (*iotago.Message, error) {
	urls := b.GetGatewayConfig().AllGatewayURLs
	for _, url := range urls {
		if message, err := ProofOfWork(url, messageBuilder); err == nil {
			return message, nil
		} else {
			log.Error("GetOutPutIDs", "err", err)
		}
	}
	return nil, tokens.ErrProofOfWork
}

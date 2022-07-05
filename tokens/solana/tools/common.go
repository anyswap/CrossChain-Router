package solanatools

import (
	"log"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

type Signer struct {
	PublicKey  string
	PrivateKey string
}

func SignAndSend(mpcConfig *mpc.Config, bridge *solana.Bridge, signers []*Signer, tx *types.Transaction) string {
	signerKeys := tx.Message.SignerKeys()
	if len(signerKeys) != len(signers) {
		log.Fatal("wrong number of signer keys")
	}

	var err error
	var calctxHash string

	msgContent, err := tx.Message.Serialize()
	if err != nil {
		log.Fatal("unable to encode message for signing", err)
	}

	for i := 0; i < len(signers); i++ {
		var signer = signers[i]
		if signer.PrivateKey != "" {
			signAccount, _ := types.AccountFromPrivateKeyBase58(signer.PrivateKey)
			signature, _ := signAccount.PrivateKey.Sign(msgContent)
			tx.Signatures = append(tx.Signatures, signature)
			if i == 0 {
				calctxHash = signature.String()
			}
		} else {
			var keyID string
			var rsvs []string

			keyID, rsvs, err = mpcConfig.DoSignOneED(signer.PublicKey, common.ToHex(msgContent[:]), "solanaChangeMPC")
			if len(rsvs) != 1 {
				log.Fatal("get sign status require one rsv but return many", err)
			}
			rsv := rsvs[0]
			sig, err := types.NewSignatureFromString(rsv)
			if err != nil {
				log.Fatal("get signature from rsv failed", "keyID", keyID, "txid", err)
			}
			tx.Signatures = append(tx.Signatures, sig)
			if i == 0 {
				calctxHash = sig.String()
			}
		}
	}

	var sendTxHash string
	for i := 0; i < 10; i++ {
		sendTxHash, err = bridge.SendTransaction(tx)
		if err != nil {
			if strings.Contains(err.Error(), "Blockhash not found") {
				time.Sleep(5 * time.Second)
			} else {
				break
			}
		}
	}
	if err != nil {
		log.Fatal("SendTransaction err", err)
	}
	if sendTxHash != calctxHash {
		log.Fatal("SendTransaction sendTxHash not same")
	}
	return sendTxHash
}

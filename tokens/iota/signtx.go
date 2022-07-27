package iota

import (
	"encoding/hex"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	iotago "github.com/iotaledger/iota.go/v2"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signTx interface{}, txHash string, err error) {
	mpcParams := params.GetMPCConfig(b.UseFastMPC)
	if mpcParams.SignWithPrivateKey {
		priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
		return b.SignTransactionWithPrivateKey(rawTx, priKey)
	}
	return
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signTx interface{}, txHash string, err error) {
	mpc := b.GetRouterContract("")
	if edAddr := ConvertStringToAddress(mpc); edAddr != nil {
		priv, _ := hex.DecodeString(privKey)
		signKey := iotago.NewAddressKeysForEd25519Address(edAddr, priv)
		signer := iotago.NewInMemoryAddressSigner(signKey)

		tx := rawTx.(*iotago.TransactionBuilder)
		if message, err := tx.BuildAndSwapToMessageBuilder(signer, nil).Build(); err == nil {
			return message, iotago.MessageIDToHexString(message.MustID()), nil
		} else {
			log.Warnf("BuildAndSwapToMessageBuilder err:%+v\n", err)
		}
	}
	return nil, "", tokens.ErrCommitMessage
}

package flow

import (
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
)

// MPCSignTransaction mpc sign raw tx
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	_, ok := rawTx.(*sdk.Transaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}

	mpcParams := params.GetMPCConfig(b.UseFastMPC)
	if mpcParams.SignWithPrivateKey {
		priKey := mpcParams.GetSignerPrivateKey(b.ChainConfig.ChainID)
		return b.SignTransactionWithPrivateKey(rawTx, priKey)
	}

	return nil, txHash, nil
}

// SignTransactionWithPrivateKey sign tx with ECDSA private key string
func (b *Bridge) SignTransactionWithPrivateKey(rawTx interface{}, privKey string) (signedTx interface{}, txHash string, err error) {
	ecPrikey, err := crypto.DecodePrivateKeyHex(crypto.ECDSA_secp256k1, privKey)
	if err != nil {
		return nil, "", err
	}
	return signTransaction(rawTx, ecPrikey)
}

func signTransaction(tx interface{}, privKey crypto.PrivateKey) (signedTx interface{}, txHash string, err error) {
	rawTx := tx.(sdk.Transaction)
	key := sdk.NewAccountKey().
		SetPublicKey(privKey.PublicKey()).
		SetSigAlgo(privKey.Algorithm()).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)
	keySigner, err := crypto.NewInMemorySigner(privKey, key.HashAlgo)
	if err != nil {
		return nil, "", err
	}
	err = rawTx.SignEnvelope(rawTx.Payer, rawTx.ProposalKey.KeyIndex, keySigner)
	if err != nil {
		return nil, "", err
	}

	return rawTx, rawTx.ID().String(), nil
}

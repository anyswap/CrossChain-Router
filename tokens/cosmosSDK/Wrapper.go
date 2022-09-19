package cosmosSDK

import (
	"github.com/cosmos/cosmos-sdk/client"
	codecTypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	errorsTypes "github.com/cosmos/cosmos-sdk/types/errors"
	txTypes "github.com/cosmos/cosmos-sdk/types/tx"
	txSigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authSigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/gogo/protobuf/proto"
)

var (
	_ authSigning.Tx             = &Wrapper{}
	_ client.TxBuilder           = &Wrapper{}
	_ ante.HasExtensionOptionsTx = &Wrapper{}
)

// wrapper is a wrapper around the tx.Tx proto.Message which retain the raw
// body and auth_info bytes.
type Wrapper struct {
	tx *txTypes.Tx

	// bodyBz represents the protobuf encoding of TxBody. This should be encoding
	// from the client using TxRaw if the tx was decoded from the wire
	bodyBz []byte

	// authInfoBz represents the protobuf encoding of TxBody. This should be encoding
	// from the client using TxRaw if the tx was decoded from the wire
	authInfoBz []byte
}

func (w *Wrapper) AddAuxSignerData(_ txTypes.AuxSignerData) error {
	return nil
}

func (w *Wrapper) SetMsgs(msgs ...types.Msg) error {
	anys, err := txTypes.SetMsgs(msgs)
	if err != nil {
		return err
	}

	w.tx.Body.Messages = anys

	// set bodyBz to nil because the cached bodyBz no longer matches tx.Body
	w.bodyBz = nil

	return nil
}

func (w *Wrapper) SetMemo(memo string) {
	w.tx.Body.Memo = memo

	// set bodyBz to nil because the cached bodyBz no longer matches tx.Body
	w.bodyBz = nil
}

func (w *Wrapper) SetFeeAmount(coins types.Coins) {
	if w.tx.AuthInfo.Fee == nil {
		w.tx.AuthInfo.Fee = &txTypes.Fee{}
	}

	w.tx.AuthInfo.Fee.Amount = coins

	// set authInfoBz to nil because the cached authInfoBz no longer matches tx.AuthInfo
	w.authInfoBz = nil
}

func (w *Wrapper) SetGasLimit(limit uint64) {
	if w.tx.AuthInfo.Fee == nil {
		w.tx.AuthInfo.Fee = &txTypes.Fee{}
	}

	w.tx.AuthInfo.Fee.GasLimit = limit

	// set authInfoBz to nil because the cached authInfoBz no longer matches tx.AuthInfo
	w.authInfoBz = nil
}

func (w *Wrapper) SetSignatures(signatures ...txSigning.SignatureV2) error {
	n := len(signatures)
	signerInfos := make([]*txTypes.SignerInfo, n)
	rawSigs := make([][]byte, n)

	for i, sig := range signatures {
		var modeInfo *txTypes.ModeInfo
		modeInfo, rawSigs[i] = authTx.SignatureDataToModeInfoAndSig(sig.Data)
		any, err := codecTypes.NewAnyWithValue(sig.PubKey)
		if err != nil {
			return err
		}
		signerInfos[i] = &txTypes.SignerInfo{
			PublicKey: any,
			ModeInfo:  modeInfo,
			Sequence:  sig.Sequence,
		}
	}

	w.setSignerInfos(signerInfos)
	w.setSignatures(rawSigs)

	return nil
}

func (w *Wrapper) setSignerInfos(infos []*txTypes.SignerInfo) {
	w.tx.AuthInfo.SignerInfos = infos
	// set authInfoBz to nil because the cached authInfoBz no longer matches tx.AuthInfo
	w.authInfoBz = nil
}

func (w *Wrapper) setSignatures(sigs [][]byte) {
	w.tx.Signatures = sigs
}

func (w *Wrapper) FeeGranter() types.AccAddress {
	feePayer := w.tx.AuthInfo.Fee.Granter
	if feePayer != "" {
		return types.MustAccAddressFromBech32(feePayer)
	}
	return nil
}

func (w *Wrapper) FeePayer() types.AccAddress {
	feePayer := w.tx.AuthInfo.Fee.Payer
	if feePayer != "" {
		return types.MustAccAddressFromBech32(feePayer)
	}
	// use first signer as default if no payer specified
	return w.GetSigners()[0]
}

func (w *Wrapper) GetSigners() []types.AccAddress {
	return w.tx.GetSigners()
}

func (w *Wrapper) GetFee() types.Coins {
	return w.tx.AuthInfo.Fee.Amount
}

func (w *Wrapper) GetGas() uint64 {
	return w.tx.AuthInfo.Fee.GasLimit
}

func (w *Wrapper) GetMemo() string {
	return w.tx.Body.Memo
}

func (w *Wrapper) GetMsgs() []types.Msg {
	return w.tx.GetMsgs()
}

func (w *Wrapper) GetPubKeys() ([]cryptoTypes.PubKey, error) {
	signerInfos := w.tx.AuthInfo.SignerInfos
	pks := make([]cryptoTypes.PubKey, len(signerInfos))

	for i, si := range signerInfos {
		// NOTE: it is okay to leave this nil if there is no PubKey in the SignerInfo.
		// PubKey's can be left unset in SignerInfo.
		if si.PublicKey == nil {
			continue
		}

		pkAny := si.PublicKey.GetCachedValue()
		pk, ok := pkAny.(cryptoTypes.PubKey)
		if ok {
			pks[i] = pk
		} else {
			return nil, errorsTypes.Wrapf(errorsTypes.ErrLogic, "Expecting PubKey, got: %T", pkAny)
		}
	}

	return pks, nil
}

func (w *Wrapper) GetSignaturesV2() ([]txSigning.SignatureV2, error) {
	signerInfos := w.tx.AuthInfo.SignerInfos
	sigs := w.tx.Signatures
	pubKeys, err := w.GetPubKeys()
	if err != nil {
		return nil, err
	}
	n := len(signerInfos)
	res := make([]txSigning.SignatureV2, n)

	for i, si := range signerInfos {
		// handle nil signatures (in case of simulation)
		if si.ModeInfo == nil {
			res[i] = txSigning.SignatureV2{
				PubKey: pubKeys[i],
			}
		} else {
			var err error
			sigData, err := authTx.ModeInfoAndSigToSignatureData(si.ModeInfo, sigs[i])
			if err != nil {
				return nil, err
			}
			res[i] = txSigning.SignatureV2{
				PubKey:   pubKeys[i],
				Data:     sigData,
				Sequence: si.GetSequence(),
			}

		}
	}

	return res, nil
}

// GetTimeoutHeight returns the transaction's timeout height (if set).
func (w *Wrapper) GetTimeoutHeight() uint64 {
	return w.tx.Body.TimeoutHeight
}

func (w *Wrapper) GetTip() *txTypes.Tip {
	return w.tx.AuthInfo.Tip
}

func (w *Wrapper) ValidateBasic() error {
	return w.tx.ValidateBasic()
}

func (w *Wrapper) GetTx() authSigning.Tx {
	return w
}

func (w *Wrapper) SetFeeGranter(feeGranter types.AccAddress) {
	if w.tx.AuthInfo.Fee == nil {
		w.tx.AuthInfo.Fee = &txTypes.Fee{}
	}

	w.tx.AuthInfo.Fee.Granter = feeGranter.String()

	// set authInfoBz to nil because the cached authInfoBz no longer matches tx.AuthInfo
	w.authInfoBz = nil
}

func (w *Wrapper) SetFeePayer(feePayer types.AccAddress) {
	if w.tx.AuthInfo.Fee == nil {
		w.tx.AuthInfo.Fee = &txTypes.Fee{}
	}

	w.tx.AuthInfo.Fee.Payer = feePayer.String()

	// set authInfoBz to nil because the cached authInfoBz no longer matches tx.AuthInfo
	w.authInfoBz = nil
}

// SetTimeoutHeight sets the transaction's height timeout.
func (w *Wrapper) SetTimeoutHeight(height uint64) {
	w.tx.Body.TimeoutHeight = height

	// set bodyBz to nil because the cached bodyBz no longer matches tx.Body
	w.bodyBz = nil
}

func (w *Wrapper) SetTip(tip *txTypes.Tip) {
	w.tx.AuthInfo.Tip = tip

	// set authInfoBz to nil because the cached authInfoBz no longer matches tx.AuthInfo
	w.authInfoBz = nil
}

func (w *Wrapper) GetExtensionOptions() []*codecTypes.Any {
	return w.tx.Body.ExtensionOptions
}

func (w *Wrapper) GetNonCriticalExtensionOptions() []*codecTypes.Any {
	return w.tx.Body.NonCriticalExtensionOptions
}

func (w *Wrapper) GetBodyBytes() []byte {
	if len(w.bodyBz) == 0 {
		// if bodyBz is empty, then marshal the body. bodyBz will generally
		// be set to nil whenever SetBody is called so the result of calling
		// this method should always return the correct bytes. Note that after
		// decoding bodyBz is derived from TxRaw so that it matches what was
		// transmitted over the wire
		var err error
		w.bodyBz, err = proto.Marshal(w.tx.Body)
		if err != nil {
			panic(err)
		}
	}
	return w.bodyBz
}

func (w *Wrapper) GetAuthInfoBytes() []byte {
	if len(w.authInfoBz) == 0 {
		// if authInfoBz is empty, then marshal the body. authInfoBz will generally
		// be set to nil whenever SetAuthInfo is called so the result of calling
		// this method should always return the correct bytes. Note that after
		// decoding authInfoBz is derived from TxRaw so that it matches what was
		// transmitted over the wire
		var err error
		w.authInfoBz, err = proto.Marshal(w.tx.AuthInfo)
		if err != nil {
			panic(err)
		}
	}
	return w.authInfoBz
}

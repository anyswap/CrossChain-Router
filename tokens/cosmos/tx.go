package cosmos

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (b *Bridge) NewSignModeHandler() signing.SignModeHandler {
	return b.TxConfig.SignModeHandler()
}

func BuildSignerData(chainID string, accountNumber, sequence uint64) signing.SignerData {
	return signing.SignerData{
		ChainID:       chainID,
		AccountNumber: accountNumber,
		Sequence:      sequence,
	}
}

func BuildSendMsg(from, to, unit string, amount *big.Int) *bankTypes.MsgSend {
	return &bankTypes.MsgSend{
		FromAddress: from,
		ToAddress:   to,
		Amount: sdk.Coins{
			sdk.NewCoin(unit, sdk.NewIntFromBigInt(amount)),
		},
	}
}

func BuildSignatures(publicKey cryptoTypes.PubKey, sequence uint64, signature []byte) signingTypes.SignatureV2 {
	return signingTypes.SignatureV2{
		PubKey: publicKey,
		Data: &signingTypes.SingleSignatureData{
			SignMode:  signingTypes.SignMode_SIGN_MODE_DIRECT,
			Signature: signature,
		},
		Sequence: sequence,
	}
}

func (b *Bridge) BuildTx(
	args *tokens.BuildTxArgs,
	to, denom, memo, publicKey string,
	amount *big.Int,
) (cosmosClient.TxBuilder, error) {
	from := args.From
	extra := args.Extra
	log.Info("start to build tx", "swapID", args.SwapID, "from", from, "to", to, "denom", denom, "memo", memo, "amount", amount, "fee", *extra.Fee, "gas", *extra.Gas, "sequence", *extra.Sequence)
	if balance, err := b.GetDenomBalance(from, denom); err != nil {
		return nil, err
	} else {
		var msgs []sdk.Msg
		if balance.BigInt().Cmp(amount) >= 0 {
			sendMsg := BuildSendMsg(from, to, denom, amount)
			msgs = append(msgs, sendMsg)
		} else {
			log.Info("balance not enough", "denom", denom, "balance", balance, "amount", amount)
			return nil, tokens.ErrBalanceNotEnough
		}

		// process charge fee on dest chain
		tokenID := args.GetTokenID()
		fromChainID := args.FromChainID
		toChainID := args.ToChainID
		if params.ChargeFeeOnDestChain(tokenID, fromChainID.String(), toChainID.String()) {
			if extra.BridgeFee != nil && extra.BridgeFee.Sign() > 0 {
				bridgeFeeReceiver := params.FeeReceiverOnDestChain(toChainID.String())
				if bridgeFeeReceiver != "" {
					sendMsg := BuildSendMsg(from, bridgeFeeReceiver, denom, extra.BridgeFee)
					msgs = append(msgs, sendMsg)
					log.Info("build charge fee on dest chain", "swapID", args.SwapID, "from", from, "receiver", bridgeFeeReceiver, "denom", denom, "amount", extra.BridgeFee)
				}
			}
		}

		txBuilder := b.TxConfig.NewTxBuilder()
		if err := txBuilder.SetMsgs(msgs...); err != nil {
			return nil, err
		}
		txBuilder.SetMemo(memo)
		if fee, err := ParseCoinsFee(*extra.Fee); err != nil {
			return nil, err
		} else {
			txBuilder.SetFeeAmount(fee)
		}
		txBuilder.SetGasLimit(*extra.Gas)
		pubKey, err := PubKeyFromStr(publicKey)
		if err != nil {
			return nil, err
		}
		sig := BuildSignatures(pubKey, *extra.Sequence, nil)
		if err := txBuilder.SetSignatures(sig); err != nil {
			return nil, err
		}
		if err := txBuilder.GetTx().ValidateBasic(); err != nil {
			return nil, err
		}

		return txBuilder, nil
	}
}

func (b *Bridge) GetSignBytes(tx *BuildRawTx) ([]byte, error) {
	handler := b.TxConfig.SignModeHandler()
	if chainName, err := b.GetChainID(); err != nil {
		return nil, err
	} else {
		txBuilder := tx.TxBuilder
		accountNumber := tx.AccountNumber
		sequence := tx.Sequence
		signerData := BuildSignerData(chainName, accountNumber, sequence)
		return handler.GetSignBytes(signingTypes.SignMode_SIGN_MODE_DIRECT, signerData, txBuilder.GetTx())
	}
}

func (b *Bridge) GetSignTx(tx signing.Tx) (signedTx []byte, txHash string, err error) {
	if txBytes, err := b.TxConfig.TxEncoder()(tx); err != nil {
		return nil, "", err
	} else {
		signedTx = []byte(base64.StdEncoding.EncodeToString(txBytes))
		txHash = fmt.Sprintf("%X", Sha256Sum(txBytes))
		log.Info("GetSignTx", "signedTx", string(signedTx), "txHash", txHash)
		return signedTx, txHash, nil
	}
}

// Sha256Sum returns the SHA256 of the data.
func Sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

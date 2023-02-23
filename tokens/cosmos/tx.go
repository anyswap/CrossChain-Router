package cosmos

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	tokenfactoryTypes "github.com/sei-protocol/sei-chain/x/tokenfactory/types"
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

func BuildCreateDenomMsg(sender, subdenom string) *tokenfactoryTypes.MsgCreateDenom {
	return tokenfactoryTypes.NewMsgCreateDenom(sender, subdenom)
}

func BuildMintMsg(sender string, amount sdk.Coin) *tokenfactoryTypes.MsgMint {
	return tokenfactoryTypes.NewMsgMint(sender, amount)
}

func BuildBurnMsg(sender string, amount sdk.Coin) *tokenfactoryTypes.MsgBurn {
	return tokenfactoryTypes.NewMsgBurn(sender, amount)
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

			if strings.Contains(denom, "/") && balance.BigInt().Cmp(amount) > 0 {
				creator, _, errt := tokenfactoryTypes.DeconstructDenom(denom)
				if errt != nil {
					return nil, errt
				}
				if creator == from {
					burnAmount := new(big.Int).Sub(balance.BigInt(), amount)
					coin := sdk.NewCoin(denom, sdk.NewIntFromBigInt(burnAmount))
					burnMsg := BuildBurnMsg(from, coin)
					msgs = append(msgs, burnMsg)
				}
			}
		} else {
			if strings.Contains(denom, "/") {
				creator, _, errt := tokenfactoryTypes.DeconstructDenom(denom)
				if errt != nil {
					return nil, errt
				}
				if creator == from {
					sendMsg := BuildSendMsg(from, to, denom, balance.BigInt())
					msgs = append(msgs, sendMsg)

					mintAmount := new(big.Int).Sub(amount, balance.BigInt())
					coin := sdk.NewCoin(denom, sdk.NewIntFromBigInt(mintAmount))
					mintMsg := BuildMintMsg(from, coin)
					msgs = append(msgs, mintMsg)
				} else {
					log.Info("balance not enough", "denom", denom, "balance", balance, "amount", amount)
					return nil, tokens.ErrBalanceNotEnough
				}
			} else {
				log.Info("balance not enough", "denom", denom, "balance", balance, "amount", amount)
				return nil, tokens.ErrBalanceNotEnough
			}
		}

		// process charge fee on dest chain
		var bridgeFeeReceiver string
		tokenID := args.GetTokenID()
		fromChainID := args.FromChainID
		toChainID := args.ToChainID
		if params.ChargeFeeOnDestChain(tokenID, fromChainID.String(), toChainID.String()) {
			if extra.BridgeFee != nil && extra.BridgeFee.Sign() > 0 {
				bridgeFeeReceiver = params.FeeReceiverOnDestChain(toChainID.String())
			}
		}
		if bridgeFeeReceiver != "" {
			var isMinted bool
			if strings.Contains(denom, "/") {
				creator, _, errt := tokenfactoryTypes.DeconstructDenom(denom)
				if errt != nil {
					return nil, errt
				}
				if creator == from {
					coin := sdk.NewCoin(denom, sdk.NewIntFromBigInt(extra.BridgeFee))
					mintMsg := BuildMintMsg(bridgeFeeReceiver, coin)
					msgs = append(msgs, mintMsg)
					isMinted = true
				}
			}
			if !isMinted {
				if balance.BigInt().Cmp(new(big.Int).Add(amount, extra.BridgeFee)) < 0 {
					log.Info("balance not enough", "denom", denom, "balance", balance, "amount", amount, "fee", extra.BridgeFee)
					return nil, tokens.ErrBalanceNotEnough
				}
				sendMsg := BuildSendMsg(from, bridgeFeeReceiver, denom, extra.BridgeFee)
				msgs = append(msgs, sendMsg)
			}
			log.Info("build charge fee on dest chain", "swapID", args.SwapID, "from", from, "receiver", bridgeFeeReceiver, "denom", denom, "fee", extra.BridgeFee)
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

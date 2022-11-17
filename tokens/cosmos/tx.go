package cosmos

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	tokenfactoryTypes "github.com/sei-protocol/sei-chain/x/tokenfactory/types"
)

const (
	SimulateTx = "/cosmos/tx/v1beta1/simulate"
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

func BuildMintMsg(sender string, amount types.Coin) *tokenfactoryTypes.MsgMint {
	return tokenfactoryTypes.NewMsgMint(sender, amount)
}

func BuildBurnMsg(sender string, amount types.Coin) *tokenfactoryTypes.MsgBurn {
	return tokenfactoryTypes.NewMsgBurn(sender, amount)
}

func BuildSendMsg(from, to, unit string, amount *big.Int) *bankTypes.MsgSend {
	return &bankTypes.MsgSend{
		FromAddress: from,
		ToAddress:   to,
		Amount: types.Coins{
			types.NewCoin(unit, types.NewIntFromBigInt(amount)),
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
	from, to, denom, memo, publicKey string,
	amount *big.Int,
	extra *tokens.AllExtras,
) (cosmosClient.TxBuilder, error) {
	if balance, err := b.GetDenomBalance(from, denom); err != nil {
		return nil, err
	} else {
		var msgs []types.Msg
		if balance >= amount.Uint64() {
			sendMsg := BuildSendMsg(from, to, denom, amount)
			msgs = append(msgs, sendMsg)

			if strings.Contains(denom, "/") && balance != amount.Uint64() {
				creater := strings.Split(denom, "/")[1]
				if creater == from {
					coin := types.NewCoin(denom, types.NewIntFromUint64(balance-amount.Uint64()))
					burnMsg := BuildBurnMsg(from, coin)
					msgs = append(msgs, burnMsg)
				}
			}
		} else {
			if strings.Contains(denom, "/") {
				creater := strings.Split(denom, "/")[1]
				if creater == from {
					coin := types.NewCoin(denom, types.NewIntFromUint64(amount.Uint64()-balance))
					mintMsg := BuildMintMsg(from, coin)
					msgs = append(msgs, mintMsg)

					sendMsg := BuildSendMsg(from, to, denom, amount)
					msgs = append(msgs, sendMsg)
				} else {
					return nil, tokens.ErrBalanceNotEnough
				}
			} else {
				return nil, tokens.ErrBalanceNotEnough
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

func (b *Bridge) SimulateTx(simulateReq *SimulateRequest) (string, error) {
	if data, err := json.Marshal(simulateReq); err != nil {
		return "", err
	} else {
		for _, url := range b.AllGatewayURLs {
			restApi := url + SimulateTx
			if res, err := client.RPCRawPostWithTimeout(restApi, string(data), 120); err == nil && res != "" && res != "\n" {
				return res, nil
			}
		}
		return "", tokens.ErrSimulateTx
	}
}

func (b *Bridge) GetSignBytes(txBuilder cosmosClient.TxBuilder, accountNumber, sequence uint64) ([]byte, error) {
	handler := b.TxConfig.SignModeHandler()
	if chainName, err := b.GetChainID(); err != nil {
		return nil, err
	} else {
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
		log.Warn("GetSignTx", "signedTx", string(signedTx), "txHash", txHash)
		return signedTx, txHash, nil
	}
}

// Sha256Sum returns the SHA256 of the data.
func Sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

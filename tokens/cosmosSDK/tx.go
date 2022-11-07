package cosmosSDK

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	ibcTypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v6/modules/core/02-client/types"
)

var (
	BroadTx       = "/cosmos/tx/v1beta1/txs"
	SimulateTx    = "/cosmos/tx/v1beta1/simulate"
	TimeoutHeight = clienttypes.NewHeight(1, 999999999)
)

func (c *CosmosRestClient) SendTransaction(signedTx interface{}) (string, error) {
	if txBytes, ok := signedTx.([]byte); !ok {
		return "", errors.New("wrong signed transaction type")
	} else {
		req := &BroadcastTxRequest{
			TxBytes: string(txBytes),
			Mode:    "BROADCAST_MODE_SYNC",
		}
		if txRes, err := c.BroadcastTx(req); err != nil {
			return "", err
		} else {
			var txResponse *BroadcastTxResponse
			if err := json.Unmarshal([]byte(txRes), &txResponse); err != nil {
				return "", err
			}
			if txResponse.TxResponse.Code != 0 && txResponse.TxResponse.Code != 19 {
				return "", fmt.Errorf(
					"SendTransaction error, code: %v, log:%v",
					txResponse.TxResponse.Code, txResponse.TxResponse.RawLog)
			}
			return txResponse.TxResponse.TxHash, nil
		}
	}
}

func (c *CosmosRestClient) BroadcastTx(req *BroadcastTxRequest) (string, error) {
	if data, err := json.Marshal(req); err != nil {
		return "", err
	} else {
		for _, url := range c.BaseUrls {
			restApi := url + BroadTx
			if res, err := client.RPCJsonPostWithTimeout(restApi, string(data), 120); err == nil {
				return res, nil
			}
		}
		return "", tokens.ErrBroadcastTx
	}
}

func (c *CosmosRestClient) NewSignModeHandler() signing.SignModeHandler {
	return c.TxConfig.SignModeHandler()
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
		Amount: types.Coins{
			types.NewCoin(unit, types.NewIntFromBigInt(amount)),
		},
	}
}

func BuildIbcTransferMsg(sourcePort, sourceChannel, sender, receiver string, coin types.Coin) *ibcTypes.MsgTransfer {
	return ibcTypes.NewMsgTransfer(sourcePort, sourceChannel, coin, sender, receiver, TimeoutHeight, 0)
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

func (c *CosmosRestClient) BuildTx(
	from, to, denom, memo, publicKey string,
	amount *big.Int,
	extra *tokens.AllExtras,
	ibcFlag bool,
	tokenCfg *tokens.TokenConfig,
) (cosmosClient.TxBuilder, error) {
	if balance, err := GetDenomBalance(from, denom); err != nil {
		return nil, err
	} else {
		var msgs []types.Msg
		if balance >= amount.Uint64() {
			if ibcFlag {
				coin := types.NewCoin(denom, types.NewInt(amount.Int64()))
				channelInfo := strings.Split(tokenCfg.Extra, ":")
				if len(channelInfo) != 2 {
					return nil, tokens.ErrChannelConfig
				}
				ibcTransferMsg := BuildIbcTransferMsg(channelInfo[1], channelInfo[0], from, to, coin)
				msgs = append(msgs, ibcTransferMsg)
			} else {
				sendMsg := BuildSendMsg(from, to, denom, amount)
				msgs = append(msgs, sendMsg)
			}
		} else {
			return nil, tokens.ErrBalanceNotEnough
		}

		txBuilder := c.TxConfig.NewTxBuilder()
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

func (c *CosmosRestClient) SimulateTx(simulateReq *SimulateRequest) (string, error) {
	if data, err := json.Marshal(simulateReq); err != nil {
		return "", err
	} else {
		for _, url := range c.BaseUrls {
			restApi := url + SimulateTx
			if res, err := client.RPCRawPostWithTimeout(restApi, string(data), 120); err == nil && res != "" && res != "\n" {
				return res, nil
			}
		}
		return "", tokens.ErrSimulateTx
	}
}

func (c *CosmosRestClient) GetSignBytes(txBuilder cosmosClient.TxBuilder, accountNumber, sequence uint64) ([]byte, error) {
	handler := c.TxConfig.SignModeHandler()
	if chainName, err := c.GetChainID(); err != nil {
		return nil, err
	} else {
		signerData := BuildSignerData(chainName, accountNumber, sequence)
		return handler.GetSignBytes(signingTypes.SignMode_SIGN_MODE_DIRECT, signerData, txBuilder.GetTx())
	}
}

func (c *CosmosRestClient) GetSignTx(tx signing.Tx) (signedTx []byte, txHash string, err error) {
	if txBytes, err := c.TxConfig.TxEncoder()(tx); err != nil {
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

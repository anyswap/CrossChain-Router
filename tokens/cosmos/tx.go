package cosmos

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	BroadTx = "/cosmos/tx/v1beta1/txs/"
)

func (c *CosmosRestClient) SendTransaction(signedTx interface{}) (string, error) {
	if txBytes, ok := signedTx.([]byte); !ok {
		return "", errors.New("wrong signed transaction type")
	} else {
		// use sync mode because block mode may rpc call timeout
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
			return txResponse.TxResponse.Txhash, nil
		}
	}
}

func (c *CosmosRestClient) BroadcastTx(req *BroadcastTxRequest) (string, error) {
	if data, err := json.Marshal(req); err != nil {
		return "", err
	} else {
		for _, url := range c.BaseUrls {
			restApi := url + BroadTx
			if res, err := client.RPCRawPost(restApi, string(data)); err == nil {
				return res, nil
			}
		}
		return "", tokens.ErrBroadcastTx
	}
}

func NewTxBuilder() cosmosClient.TxBuilder {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authTx.NewTxConfig(protoCodec, authTx.DefaultSignModes)
	return txConfig.NewTxBuilder()
}

// func NewTxBuilder() *Wrapper {
// 	return &Wrapper{
// 		tx: &tx.Tx{
// 			Body: &tx.TxBody{},
// 			AuthInfo: &tx.AuthInfo{
// 				Fee: &tx.Fee{},
// 			},
// 		},
// 	}
// }

func BuildSendMgs(from, to, unit string, amount *big.Int) *banktypes.MsgSend {
	return &banktypes.MsgSend{
		FromAddress: from,
		ToAddress:   to,
		Amount: sdk.Coins{
			sdk.NewCoin(unit, sdk.NewIntFromBigInt(amount)),
		},
	}
}

func BuildSignatures(publicKey types.PubKey, sequence uint64, signature []byte) signing.SignatureV2 {
	return signing.SignatureV2{
		PubKey: publicKey,
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
			Signature: signature,
		},
		Sequence: sequence,
	}
}

func BuildTx(
	from, to, CoinSymbol, memo string,
	amount *big.Int,
	extra *tokens.AllExtras,
	publicKey string,
) (cosmosClient.TxBuilder, error) {
	txBuilder := NewTxBuilder()
	msg := BuildSendMgs(from, to, CoinSymbol, amount)
	txBuilder.SetMsgs(msg)
	txBuilder.SetMemo(memo)
	if fee, err := ParseCoinsFee(*extra.Fee); err != nil {
		return nil, err
	} else {
		txBuilder.SetFeeAmount(fee)
	}
	txBuilder.SetGasLimit(*extra.Gas)
	if pubKey, err := PubKeyFromStr(publicKey); err != nil {
		return nil, err
	} else {
		sig := BuildSignatures(pubKey, *extra.Sequence, nil)
		txBuilder.SetSignatures(sig)
	}
	if err := txBuilder.GetTx().ValidateBasic(); err != nil {
		return nil, err
	}
	return txBuilder, nil
}

// func BuildTx(
// 	from, to, CoinSymbol, memo string,
// 	amount *big.Int,
// 	extra *tokens.AllExtras,
// 	publicKey string,
// ) (*Wrapper, error) {
// 	txBuilder := NewTxBuilder()
// 	msg := BuildSendMgs(from, to, CoinSymbol, amount)
// 	txBuilder.SetMsgs(msg)
// 	txBuilder.SetMemo(memo)
// 	if fee, err := ParseCoinsFee(*extra.Fee); err != nil {
// 		return nil, err
// 	} else {
// 		txBuilder.SetFeeAmount(fee)
// 	}
// 	txBuilder.SetGasLimit(*extra.Gas)
// 	if pubKey, err := PubKeyFromStr(publicKey); err != nil {
// 		return nil, err
// 	} else {
// 		sig := BuildSignatures(pubKey, *extra.Sequence)
// 		txBuilder.SetSignatures(sig)
// 	}
// 	if err := txBuilder.ValidateBasic(); err != nil {
// 		return nil, err
// 	}
// 	return txBuilder, nil
// }

func GetTxDataBytes(txBuilder cosmosClient.TxBuilder) ([]byte, error) {
	encoder := authTx.DefaultTxEncoder()
	if txBz, err := encoder(txBuilder.GetTx()); err != nil {
		return nil, err
	} else {
		return txBz, nil
	}
}

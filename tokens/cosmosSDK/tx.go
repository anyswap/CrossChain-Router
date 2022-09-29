package cosmosSDK

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	BroadTx    = "/cosmos/tx/v1beta1/txs"
	SimulateTx = "/cosmos/tx/v1beta1/simulate"
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
			if res, err := client.RPCRawPostWithTimeout(restApi, string(data), 120); err == nil && res != "" && res != "\n" {
				return res, nil
			}
		}
		return "", tokens.ErrBroadcastTx
	}
}

func (c *CosmosRestClient) NewSignModeHandler() signing.SignModeHandler {
	return c.TxConfig.SignModeHandler()
}

func BuildSignerData(address, chainID string, accountNumber, sequence uint64, pubKey cryptoTypes.PubKey) signing.SignerData {
	return signing.SignerData{
		Address:       address,
		ChainID:       chainID,
		AccountNumber: accountNumber,
		Sequence:      sequence,
		PubKey:        pubKey,
	}
}

func BuildSendMgs(from, to, unit string, amount *big.Int) *bankTypes.MsgSend {
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

func (c *CosmosRestClient) BuildTx(
	from, to, denom, memo, publicKey string,
	amount *big.Int,
	extra *tokens.AllExtras,
) (cosmosClient.TxBuilder, error) {
	txBuilder := c.TxConfig.NewTxBuilder()
	msg := BuildSendMgs(from, to, denom, amount)
	if err := txBuilder.SetMsgs(msg); err != nil {
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

func (c *CosmosRestClient) GetSignBytes(txBuilder cosmosClient.TxBuilder, account string, accountNumber, sequence uint64, pubKey cryptoTypes.PubKey) ([]byte, error) {
	handler := c.TxConfig.SignModeHandler()
	if chainName, err := c.GetChainID(); err != nil {
		return nil, err
	} else {
		signerData := BuildSignerData(account, chainName, accountNumber, sequence, pubKey)
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

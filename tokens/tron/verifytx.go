package tron

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	proto "github.com/golang/protobuf/proto"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
)

// GetTransactionStatus returns tx status
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = &tokens.TxStatus{}
	rpcError := &RPCError{[]error{}, "GetTransactionStatus"}
	defer func() {
		if r := recover(); r != nil {
			rpcError.log(fmt.Errorf("%v", r))
		}
	}()
	txinfo := make(map[string]interface{})
	for _, endpoint := range b.GatewayConfig.APIAddress {
		apiurl := strings.TrimSuffix(endpoint, "/") + `/walletsolidity/gettransactioninfobyid`
		res, err := post(apiurl, `{"value":"`+txHash+`"}`)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(res, &txinfo)
		if err != nil {
			rpcError.log(err)
			continue
		}
	}

	defer func() {
		if r := recover(); r != nil {
			status = nil
			err = fmt.Errorf("%v", r)
		}
	}()

	if txinfo["result"].(string) != "SUCCESS" {
		return nil, errors.New("tron tx not success")
	}
	cres := txinfo["contractResult"].([]interface{})
	if len(cres) < 1 {
		return nil, errors.New("tron tx no result")
	}
	for _, cr := range cres {
		r := []byte(cr.(string))
		if len(r) > 0 && new(big.Int).SetBytes(r).Int64() != 1 {
			return nil, errors.New("tron tx wrong result")
		}
	}

	status.Receipt = txinfo
	status.BlockHeight = uint64(txinfo["blockNumber"].(float64))
	status.BlockTime = uint64(txinfo["blockTimeStamp"].(float64)) / 1000

	if latest, err := b.GetLatestBlockNumber(); err == nil {
		status.Confirmations = latest - status.BlockHeight
	}
	status.CustomeCheckStable = func(confirmations uint64) bool {
		return status.Confirmations >= confirmations
	}
	return
}

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) error {
	tx, ok := rawTx.(*core.Transaction)
	if !ok {
		return errors.New("wrong raw tx param")
	}
	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	msgHash := msgHashes[0]
	sigHash := CalcTxHash(tx)
	if !strings.EqualFold(sigHash, msgHash) {
		log.Trace("message hash mismatch", "want", msgHash, "have", sigHash)
		return tokens.ErrMsgHashMismatch
	}
	return nil
}

// VerifyTransaction api
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifyERC20SwapTx(txHash, logIndex, allowUnstable)
	case tokens.NFTSwapType:
		return b.verifyNFTSwapTx(txHash, logIndex, allowUnstable)
	case tokens.AnyCallSwapType:
		return b.verifyAnyCallSwapTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

func CalcTxHash(tx *core.Transaction) string {
	rawData, err := proto.Marshal(tx.GetRawData())
	if err != nil {
		return ""
	}

	h256h := sha256.New()
	h256h.Write(rawData)
	hash := h256h.Sum(nil)
	txhash := common.ToHex(hash)

	txhash = strings.TrimPrefix(txhash, "0x")
	return txhash
}

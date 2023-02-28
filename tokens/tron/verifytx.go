package tron

import (
	"crypto/sha256"
	"errors"
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
)

// GetTransactionStatus returns tx status
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	txInfo, err := b.GetTransactionInfo(txHash)
	if err != nil {
		return nil, err
	}

	status = &tokens.TxStatus{}

	status.Receipt = txInfo
	status.Failed = !txInfo.IsStatusOk()
	status.BlockHeight = txInfo.BlockNumber
	status.BlockTime = txInfo.BlockTimeStamp
	status.BlockHash = txInfo.TxID

	if latest, err := b.GetLatestBlockNumber(); err == nil {
		status.Confirmations = latest - status.BlockHeight
	}
	return status, nil
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
	_, err = h256h.Write(rawData)
	if err != nil {
		return ""
	}
	hash := h256h.Sum(nil)
	txhash := common.BytesToHexString(hash)

	txhash = strings.TrimPrefix(txhash, "0x")
	return txhash
}

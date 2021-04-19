package eth

import (
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/tokens"
	"github.com/anyswap/CrossChain-Router/types"
)

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (interface{}, error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string, withExt bool) *tokens.TxStatus {
	var txStatus tokens.TxStatus
	txr, url, err := b.GetTransactionReceipt(txHash, withExt)
	if err != nil {
		log.Trace("GetTransactionReceipt fail", "hash", txHash, "err", err)
		return &txStatus
	}
	txStatus.BlockHeight = txr.BlockNumber.ToInt().Uint64()
	txStatus.BlockHash = txr.BlockHash.String()
	if txStatus.BlockHeight != 0 {
		urls := []string{url}
		latest, err := getLatestBlockNumber(urls)
		if err == nil {
			if latest > txStatus.BlockHeight {
				txStatus.Confirmations = latest - txStatus.BlockHeight
			}
		} else {
			log.Debug("GetLatestBlockNumber fail", "url", url, "err", err)
		}
		block, err := getBlockByHash(txStatus.BlockHash, urls)
		if err == nil {
			txStatus.BlockTime = block.Time.ToInt().Uint64()
		} else {
			log.Debug("GetBlockByHash fail", "hash", txStatus.BlockHash, "url", url, "err", err)
		}
	}
	txStatus.Receipt = txr
	return &txStatus
}

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) error {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	if len(msgHashes) != 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	msgHash := msgHashes[0]
	signer := b.Signer
	sigHash := signer.Hash(tx)
	if sigHash.String() != msgHash {
		log.Trace("message hash mismatch", "want", msgHash, "have", sigHash.String())
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
	case tokens.RouterSwapType:
		return b.verifyRouterSwapTx(txHash, logIndex, allowUnstable)
	case tokens.AnyCallSwapType:
		return b.verifyAnyCallSwapTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

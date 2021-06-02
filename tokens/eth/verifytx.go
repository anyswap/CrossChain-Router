package eth

import (
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/tokens"
	"github.com/anyswap/CrossChain-Router/types"
)

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) *tokens.TxStatus {
	var txStatus tokens.TxStatus
	txr, url, err := b.GetTransactionReceipt(txHash)
	if err != nil {
		return &txStatus
	}
	txStatus.BlockHeight = txr.BlockNumber.ToInt().Uint64()
	txStatus.BlockHash = txr.BlockHash.String()
	block, err := b.GetBlockByHash(txStatus.BlockHash)
	if err == nil {
		txStatus.BlockTime = block.Time.ToInt().Uint64()
	} else {
		log.Debug("GetBlockByHash fail", "hash", txStatus.BlockHash, "err", err)
	}
	if txStatus.BlockHeight != 0 {
		latest, err := b.GetLatestBlockNumberOf(url)
		if err == nil {
			if latest > txStatus.BlockHeight {
				txStatus.Confirmations = latest - txStatus.BlockHeight
			}
		} else {
			log.Debug("GetLatestBlockNumber fail", "err", err)
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

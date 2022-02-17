package solana

import (
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (*tokens.TxStatus, error) {
	tx, err := b.GetTransaction(txHash)
	if err != nil {
		return nil, err
	}
	txm, ok := tx.(*types.TransactionWithMeta)
	if !ok {
		return nil, tokens.ErrWrongRawTx
	}

	var txStatus tokens.TxStatus

	txStatus.Sender = txm.Transaction.Message.AccountKeys[0].String()
	txStatus.BlockHeight = uint64(txm.Slot)
	txStatus.BlockHash = txm.Transaction.Message.RecentBlockhash.String()
	txStatus.BlockTime = uint64(txm.BlockTime)

	if txStatus.BlockHeight != 0 {
		for i := 0; i < 3; i++ {
			latest, errt := b.GetLatestBlockNumber()
			if errt == nil {
				if latest > txStatus.BlockHeight {
					txStatus.Confirmations = latest - txStatus.BlockHeight
				}
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	txStatus.Receipt = txm.Meta
	return &txStatus, nil
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (result interface{}, err error) {
	obj := map[string]interface{}{
		"encoding":   "json",
		"commitment": "finalized",
	}
	callMethod := "getTransaction"
	gateway := b.GatewayConfig
	var tx types.TransactionWithMeta
	err = RPCCall(&tx, gateway.APIAddress, callMethod, txHash, obj)
	if err != nil && tokens.IsRPCQueryOrNotFoundError(err) && len(gateway.APIAddressExt) > 0 {
		err = RPCCall(&tx, gateway.APIAddressExt, callMethod, txHash, obj)
	}
	if err != nil {
		return nil, err
	}
	if uint64(tx.Slot) == 0 {
		return nil, tokens.ErrTxNotFound
	}
	return &tx, nil
}

func (b *Bridge) getTransactionMeta(swapInfo *tokens.SwapTxInfo, allowUnstable bool) (*types.TransactionMeta, error) {
	txStatus, err := b.GetTransactionStatus(swapInfo.Hash)
	if err != nil {
		log.Error("get tx meta failed", "hash", swapInfo.Hash, "err", err)
		return nil, err
	}
	if txStatus == nil || txStatus.BlockHeight == 0 {
		return nil, tokens.ErrTxNotFound
	}
	if txStatus.BlockHeight < b.ChainConfig.InitialHeight {
		return nil, tokens.ErrTxBeforeInitialHeight
	}

	swapInfo.From = txStatus.Sender         // From
	swapInfo.Height = txStatus.BlockHeight  // Height
	swapInfo.Timestamp = txStatus.BlockTime // Timestamp

	if !allowUnstable && txStatus.Confirmations < b.ChainConfig.Confirmations {
		return nil, tokens.ErrTxNotStable
	}

	txm, ok := txStatus.Receipt.(*types.TransactionMeta)
	if !ok || !txm.IsStatusOk() {
		return txm, tokens.ErrTxWithWrongStatus
	}

	return txm, nil
}

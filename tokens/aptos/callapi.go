package aptos

import (
	"fmt"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return 0, fmt.Errorf("Aptos GetAccount, %w", err)
	}
	return strconv.ParseUint(account.SequenceNumber, 10, 64)
}

func (b *Bridge) GetLatestBlockNumber() (num uint64, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		resp, err1 := b.GetLedger()
		if err1 != nil || resp == nil {
			err = err1
			log.Warn("Try get latest block number failed", "error", err1)
			continue
		}
		return strconv.ParseUint(resp.LedgerVersion, 10, 64)
	}
	return
}

func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (num uint64, err error) {
	client := RestClient{
		Url: apiAddress,
	}
	for i := 0; i < rpcRetryTimes; i++ {
		resp, err1 := client.GetLedger()
		if err1 != nil || resp == nil {
			err = err1
			log.Warn("Try get latest block number failed", "error", err1)
			continue
		}
		return strconv.ParseUint(resp.LedgerVersion, 10, 64)
	}
	return
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		resp, err1 := b.GetTransactions(txHash)
		if err1 != nil || resp == nil {
			log.Warn("Try get transaction failed", "error", err1)
			err = err1
			continue
		}
		tx = resp
		return
	}
	return
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)
	tx, err := b.GetTransaction(txHash)
	if err != nil {
		return nil, err
	}

	txres, ok := tx.(*TransactionInfo)
	if !ok {
		// unexpected
		log.Warn("Aptos GetTransactionStatus", "error", tokens.ErrTxResultType)
		return nil, tokens.ErrTxResultType
	}

	// Check tx status
	if !txres.Success {
		log.Warn("Aptos tx status is not success", "result", txres.Success)
		return nil, tokens.ErrTxWithWrongStatus
	}

	status.Receipt = nil
	inledger, err := strconv.ParseUint(txres.Version, 10, 64)
	if err != nil {
		return nil, err
	}
	status.BlockHeight = inledger

	if latest, err := b.GetLatestBlockNumber(); err == nil && latest > uint64(inledger) {
		status.Confirmations = latest - uint64(inledger)
	}
	return
}

package solana

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

var (
	sendTxOpts *types.SendTransactionOptions
)

// SendTransaction impl
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(*types.Transaction)
	if !ok {
		return "", errors.New("wrong signed transaction type")
	}
	txHash, err = b.SendSignedTransaction(tx, sendTxOpts)
	if err != nil {
		log.Info("Solana SendTransaction failed", "err", err)
	} else {
		log.Info("Solana SendTransaction success", "hash", txHash)

	}
	return txHash, err
}

// SendSignedTransaction call sendTransaction
func (b *Bridge) SendSignedTransaction(tx *types.Transaction, opts *types.SendTransactionOptions) (txHash string, err error) {
	txData, err := tx.SerializeAll()
	if err != nil {
		return "", err
	}
	b64TxData := base64.StdEncoding.EncodeToString(txData)

	log.Debug("SendSignedTransaction: ", "length", len(txData), "b64TxData: ", b64TxData)

	obj := map[string]interface{}{
		"encoding":   "base64",
		"commitment": "confirmed",
	}
	if opts != nil {
		if opts.SkipPreflight {
			obj["skipPreflight"] = opts.SkipPreflight
		}
		// It is recommended to specify the same commitment
		// and preflight commitment to avoid confusing behavior.
		if opts.PreflightCommitment != "" {
			obj["preflightCommitment"] = opts.PreflightCommitment
			obj["commitment"] = opts.PreflightCommitment
		}
	}

	sendTxParams := []interface{}{b64TxData, obj}

	gateway := b.GatewayConfig
	if len(gateway.APIAddressExt) > 0 {
		txHash, err = sendRawTransaction(sendTxParams, gateway.APIAddressExt)
	} else {
		txHash, err = sendRawTransaction(sendTxParams, gateway.APIAddress)
	}
	if txHash != "" {
		return txHash, nil
	}
	return "", err
}

func sendRawTransaction(sendTxParams []interface{}, urls []string) (txHash string, err error) {
	logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
	var result string
	// the blockhash is ahead of blockchain when get,so need to retry wait for the blockhash in avaliable on solana
	for i := 0; i < 5; i++ {
		url := urls[rand.Intn(len(urls))]
		err = client.RPCPost(&result, url, "sendTransaction", sendTxParams...)
		if err != nil {
			if strings.Contains(err.Error(), "Blockhash not found") {
				logFunc("solana sendRawTransaction: Blockhash not found, wait 5 sec retry", "retry times", i+1)
				time.Sleep(5 * time.Second)
				continue
			} else {
				logFunc("SendSignedTransaction failed", "url", url, "err", err)
				break
			}
		} else {
			logFunc("SendSignedTransaction success", "txHash", result, "url", url)
			txHash = result
			break
		}
	}

	if txHash != "" {
		return txHash, nil
	}
	return "", wrapRPCQueryError(err, "sendTransaction")
}

// SimulateTransaction simulate tx
func (b *Bridge) SimulateTransaction(tx *types.Transaction) (result *types.SimulateTransactionResponse, err error) {
	signData, err := tx.Message.Serialize()
	if err != nil {
		return nil, fmt.Errorf("simulate tx encode tx error: %w", err)
	}
	wireTransaction, err := tx.Serialize(signData)
	if err != nil {
		return nil, fmt.Errorf("simulate tx encode tx error: %w", err)
	}
	b64TxData := base64.StdEncoding.EncodeToString(wireTransaction)

	log.Debug("simulateTx: ", "length", len(wireTransaction), "b64TxData: ", b64TxData)
	obj := map[string]interface{}{
		"encoding":   "base64",
		"commitment": "confirmed",
		"sigVerify":  false,
	}
	sendTxParams := []interface{}{b64TxData, obj}

	gateway := b.GatewayConfig
	result, err = simulateTx(sendTxParams, gateway.APIAddress)
	if err == nil {
		log.Info("simulateTx", "success")
		return result, nil
	}
	if len(gateway.APIAddressExt) > 0 {
		result, err = simulateTx(sendTxParams, gateway.APIAddressExt)
		if err == nil {
			log.Info("simulateTx", "success")
			return result, nil
		}
	}
	return nil, err
}

func simulateTx(sendTxParams []interface{}, urls []string) (result *types.SimulateTransactionResponse, err error) {
	callMethod := "simulateTransaction"
	err = RPCCall(&result, urls, callMethod, sendTxParams...)
	return result, err
}

// GetSignatureStatuses get signature statuses
func (b *Bridge) GetSignatureStatuses(sigs []string, searchTransactionHistory bool) (result *types.GetSignatureStatusesResult, err error) {
	callMethod := "getSignatureStatuses"
	obj := map[string]interface{}{
		"searchTransactionHistory": searchTransactionHistory,
	}
	err = RPCCall(&result, b.GatewayConfig.APIAddress, callMethod, sigs, obj)
	if err == nil {
		return result, nil
	}
	err = RPCCall(&result, b.GatewayConfig.APIAddressExt, callMethod, sigs, obj)
	if err == nil {
		return result, nil
	}
	return nil, err
}

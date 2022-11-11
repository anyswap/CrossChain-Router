package near

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	rpcTimeout = 60
)

const (
	blockMethod             = "block"
	txMethod                = "tx"
	queryMethod             = "query"
	broadcastTxCommitMethod = "broadcast_tx_commit"
)

// SetRPCTimeout set rpc timeout
func SetRPCTimeout(timeout int) {
	rpcTimeout = timeout
}

func GetBlockNumberByHash(url, hash string) (uint64, error) {
	request := &client.Request{}
	request.Method = blockMethod
	request.Params = map[string]string{"block_id": hash}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result BlockDetail
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return 0, err
	}
	return result.Header.Height, nil
}

func GetLatestBlockHash(url string) (string, error) {
	request := &client.Request{}
	request.Method = blockMethod
	request.Params = map[string]string{"finality": "final"}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result BlockDetail
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return "", err
	}
	return result.Header.Hash, nil
}

// GetLatestBlockNumber get latest block height
func GetLatestBlockNumber(url string) (uint64, error) {
	request := &client.Request{}
	request.Method = blockMethod
	request.Params = map[string]string{"finality": "final"}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result BlockDetail
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return 0, err
	}
	return result.Header.Height, nil
}

// GetTransactionByHash get tx by hash
func GetTransactionByHash(url, txHash, senderID string) (*TransactionResult, error) {
	request := &client.Request{}
	request.Method = txMethod
	request.Params = []string{txHash, senderID}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result TransactionResult
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		log.Info("GetTransactionByHash", "err", err, "txHash", txHash)
		return nil, err
	}
	if !strings.EqualFold(result.Transaction.Hash, txHash) {
		return nil, fmt.Errorf("get tx hash mismatch, have %v want %v", result.Transaction.Hash, txHash)
	}
	return &result, nil
}

// GetLatestBlockNumber get latest block height
func GetAccountNonce(url, account, publicKey string) (uint64, error) {
	request := &client.Request{}
	request.Method = queryMethod
	request.Params = map[string]string{"request_type": "view_access_key", "finality": "final", "account_id": account, "public_key": publicKey}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result map[string]interface{}
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return 0, err
	}
	if result["nonce"] == nil {
		return 0, tokens.ErrGetAccountNonce
	}
	return uint64(result["nonce"].(float64)) + 1, nil
}

func BroadcastTxCommit(url string, signedTx []byte) (string, error) {
	log.Info("BroadcastTxCommit", "url", url, "signedTx", base64.StdEncoding.EncodeToString(signedTx))
	request := &client.Request{}
	request.Method = broadcastTxCommitMethod
	request.Params = []string{base64.StdEncoding.EncodeToString(signedTx)}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result TransactionResult
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return "", err
	}
	return result.Transaction.Hash, nil
}

func functionCall(url, accountID, methodName, args string) ([]byte, error) {
	request := &client.Request{}
	request.Method = queryMethod
	request.Params = map[string]string{"request_type": "call_function", "finality": "final", "account_id": accountID, "method_name": methodName, "args_base64": args}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result FunctionCallResult
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return nil, err
	}
	if result.Result == nil {
		return nil, errors.New(result.Error)
	}
	return result.Result, nil
}

func CheckTokenBalance(url, token, account, amount string) error {
	argsStr := fmt.Sprintf("{\"account_id\":\"%s\"}", account)
	argsBase64 := base64.StdEncoding.EncodeToString([]byte(argsStr))
	result, err := functionCall(url, token, GetFtBalance, argsBase64)
	if err == nil {
		if amountBigInt, err := common.GetBigIntFromStr(amount); err != nil {
			return err
		} else {
			balanceStr := string(result)
			if balance, err := common.GetBigIntFromStr(balanceStr[1 : len(balanceStr)-1]); err != nil {
				return err
			} else if balance.Cmp(amountBigInt) >= 0 {
				return nil
			} else {
				return tokens.ErrTokenBalanceNotEnough
			}
		}
	}
	return tokens.ErrQueryTokenBalance
}

func CheckBalance(url, account, amount string) error {
	request := &client.Request{}
	request.Method = queryMethod
	request.Params = map[string]string{"request_type": "view_account", "finality": "final", "account_id": account}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result map[string]interface{}
	err := client.RPCPostRequest(url, request, &result)
	if err == nil {
		if amountBigInt, err := common.GetBigIntFromStr(amount); err != nil {
			return err
		} else {
			if result["amount"] != nil {
				balanceStr := string(result["amount"].(string))
				if balance, err := common.GetBigIntFromStr(balanceStr); err != nil {
					return err
				} else if balance.Cmp(amountBigInt) >= 0 {
					return nil
				} else {
					return tokens.ErrTokenBalanceNotEnough
				}
			}
			return tokens.ErrQueryTokenBalance
		}
	}
	return err
}

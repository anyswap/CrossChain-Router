package aptos

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

var (
	API_VERSION             = "/v1/"
	AccountPath             = API_VERSION + "accounts/{address}"
	GetTransactionsPath     = API_VERSION + "transactions/by_hash/{txn_hash}"
	GetSigningMessagePath   = API_VERSION + "transactions/encode_submission"
	SubmitTranscationPath   = API_VERSION + "transactions"
	SimulateTranscationPath = API_VERSION + "transactions/simulate"

	EstimateGasPricePath = API_VERSION + "estimate_gas_price"

	SCRIPT_FUNCTION_PAYLOAD = "entry_function_payload"

	SPLIT_SYMBOL = "::"

	NATIVE_COIN     = "0x1::aptos_coin::AptosCoin"
	NATIVE_TRANSFER = "0x1::aptos_account::transfer"
	// NATIVE_TRANSFER = "0x1::coin::transfer"

	SUCCESS_HTTP_STATUS_CODE = map[int]bool{200: true, 202: true}
)

type RestClient struct {
	Url     string
	Timeout int
}

type AptosError struct {
	Message   string `json:"message"`
	ErrCode   string `json:"error_code"`
	VmErrCode int    `json:"vm_error_code,omitempty"`
}

func getAptosError(url string, body []byte) error {
	var aptErr AptosError
	if err := json.Unmarshal(body, &aptErr); err == nil {
		log.Trace("aptos rpc error", "url", url, "message", aptErr.Message, "error_code", aptErr.ErrCode, "vm_error_code", aptErr.VmErrCode)
		return fmt.Errorf("AptosError: %v", aptErr.ErrCode)
	}
	return nil
}

func (c *RestClient) GetRequest(result interface{}, uri string, params map[string]string) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	for key, val := range params {
		uri = strings.Replace(uri, "{"+key+"}", url.QueryEscape(val), -1)
	}
	errBody, err := client.RPCGetRequest2(result, c.Url+uri, nil, headers, c.Timeout)
	if err != nil && errBody != nil {
		if aptErr := getAptosError(c.Url+uri, errBody); aptErr != nil {
			return aptErr
		}
	}
	return err
}

func (c *RestClient) PostRequest(result interface{}, uri string, body interface{}) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	errBody, err := client.RPCPostBody(c.Url+uri, nil, headers, body, result, c.Timeout, SUCCESS_HTTP_STATUS_CODE)
	if err != nil && errBody != nil {
		if aptErr := getAptosError(c.Url+uri, errBody); aptErr != nil {
			return aptErr
		}
	}
	return err
}

func (c *RestClient) GetLedger() (*LedgerInfo, error) {
	resp := LedgerInfo{}
	err := c.GetRequest(&resp, API_VERSION, nil)
	return &resp, err
}

func (c *RestClient) GetAccount(address string) (*AccountInfo, error) {
	resp := AccountInfo{}
	param := map[string]string{
		"address": address,
	}
	err := c.GetRequest(&resp, AccountPath, param)
	return &resp, err
}

func (c *RestClient) GetTransactions(txHash string) (*TransactionInfo, error) {
	resp := TransactionInfo{}
	param := map[string]string{
		"txn_hash": txHash,
	}
	err := c.GetRequest(&resp, GetTransactionsPath, param)
	return &resp, err
}

func (c *RestClient) GetSigningMessage(request interface{}) (*string, error) {
	resp := ""
	err := c.PostRequest(&resp, GetSigningMessagePath, request)
	return &resp, err
}

func (c *RestClient) SubmitTranscation(request interface{}) (*TransactionInfo, error) {
	resp := TransactionInfo{}
	err := c.PostRequest(&resp, SubmitTranscationPath, request)
	return &resp, err
}

func (c *RestClient) SimulateTranscation(request interface{}, publikKey string) error {
	tx, ok := request.(*Transaction)
	if !ok {
		return fmt.Errorf("not aptos Transaction")
	}
	tx.Signature = &TransactionSignature{
		Type:      "ed25519_signature",
		PublicKey: publikKey,
		Signature: common.ToHex(make([]byte, 64)),
	}
	resp := []TransactionInfo{}
	err := c.PostRequest(&resp, SimulateTranscationPath, tx)
	if err != nil {
		return err
	}
	if len(resp) <= 0 {
		return fmt.Errorf("SimulateTranscation with no result")
	}
	if !resp[0].Success {
		result, _ := json.Marshal(resp[0])
		return fmt.Errorf("SimulateTranscation fals %s", string(result))
	}
	return err
}

func (c *RestClient) EstimateGasPrice() (*GasEstimate, error) {
	resp := GasEstimate{}
	err := c.PostRequest(&resp, EstimateGasPricePath, nil)
	return &resp, err
}

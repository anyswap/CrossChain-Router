package aptos

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

var (
	API_VERSION             = "/v1/"
	AccountPath             = API_VERSION + "accounts/{address}"
	AccountResourcePath     = API_VERSION + "accounts/{address}/resource/{resource_type}"
	GetTransactionsPath     = API_VERSION + "transactions/by_hash/{txn_hash}"
	GetSigningMessagePath   = API_VERSION + "transactions/encode_submission"
	SubmitTranscationPath   = API_VERSION + "transactions"
	SimulateTranscationPath = API_VERSION + "transactions/simulate"

	SCRIPT_FUNCTION_PAYLOAD = "entry_function_payload"

	SPLIT_SYMBOL  = "::"
	CONTRACT_NAME = "Router"

	CONTRACT_FUNC_SWAPIN  = "swapin"
	CONTRACT_FUNC_SWAPOUT = "swapout"

	NATIVE_COIN = "0x1::aptos_coin::AptosCoin"

	COIN_INFO_PREFIX = "0x1::coin::CoinInfo<%s>"

	SUCCESS_HTTP_STATUS_CODE = map[int]bool{200: true, 202: true}
)

type RestClient struct {
	Url     string
	Timeout int
}

func (c *RestClient) GetRequest(result interface{}, uri string, params map[string]string) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	for key, val := range params {
		uri = strings.Replace(uri, "{"+key+"}", url.QueryEscape(val), -1)
	}
	return client.RPCGetRequest(result, c.Url+uri, nil, headers, c.Timeout)
}

func (c *RestClient) PostRequest(result interface{}, uri string, body interface{}) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	return client.RPCPostBody(c.Url+uri, nil, headers, body, result, c.Timeout, SUCCESS_HTTP_STATUS_CODE)
}

func (c *RestClient) GetLedger() (*LedgerInfo, error) {
	resp := LedgerInfo{}
	err := c.GetRequest(&resp, API_VERSION, nil)
	return &resp, err
}

func (c *RestClient) GetAccountCoin(address, coinType string) (*CoinStoreResource, error) {
	resp := CoinStoreResource{}
	param := map[string]string{
		"address":       address,
		"resource_type": fmt.Sprintf("0x1::coin::CoinStore<%s>", coinType),
	}
	err := c.GetRequest(&resp, AccountResourcePath, param)
	return &resp, err
}

func (c *RestClient) GetAccountResource(address, resourceType string) (*CoinInfoResource, error) {
	resp := CoinInfoResource{}
	param := map[string]string{
		"address":       address,
		"resource_type": resourceType,
	}
	err := c.GetRequest(&resp, AccountResourcePath, param)
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

func (c *RestClient) GetTransactionsNotPending(txHash string) (*TransactionInfo, error) {
	resp := TransactionInfo{}
	param := map[string]string{
		"txn_hash": txHash,
	}
	err := c.GetRequest(&resp, GetTransactionsPath, param)
	return &resp, err
}

func (c *RestClient) GetSigningMessage(request *Transaction) (*string, error) {
	resp := ""
	err := c.PostRequest(&resp, GetSigningMessagePath, request)
	return &resp, err
}

func (c *RestClient) SubmitTranscation(request *Transaction) (*TransactionInfo, error) {
	resp := TransactionInfo{}
	err := c.PostRequest(&resp, SubmitTranscationPath, request)
	return &resp, err
}

func (c *RestClient) SimulateTranscation(request *Transaction) (*TransactionInfo, error) {
	resp := TransactionInfo{}
	err := c.PostRequest(&resp, SimulateTranscationPath, request)
	return &resp, err
}

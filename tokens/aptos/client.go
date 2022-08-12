package aptos

import (
	"net/url"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

var (
	AccountPath           = "/accounts/{address}"
	AccountResourcePath   = "/accounts/{address}/resource/{resource_type}"
	GetTransactionsPath   = "/transactions/{txn_hash}"
	GetSigningMessagePath = "/transactions/signing_message"
	SubmitTranscationPath = "/transactions"
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
	return client.RPCPostBody(c.Url+uri, nil, headers, body, result, c.Timeout)
}

func (c *RestClient) GetLedger() (*LedgerInfo, error) {
	resp := LedgerInfo{}
	err := c.GetRequest(&resp, "/", nil)
	return &resp, err
}

func (c *RestClient) GetAccountCoinStore(address, resourceType string) (*CoinStoreResource, error) {
	resp := CoinStoreResource{}
	param := map[string]string{
		"address":       address,
		"resource_type": resourceType,
	}
	err := c.GetRequest(&resp, AccountResourcePath, param)
	return &resp, err
}

func (c *RestClient) GetAccountCoin(address, resourceType string) (*CoinInfoResource, error) {
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

func (c *RestClient) GetSigningMessage(request *Transaction) (*SigningMessage, error) {
	resp := SigningMessage{}
	err := c.PostRequest(&resp, GetSigningMessagePath, request)
	return &resp, err
}

func (c *RestClient) SubmitTranscation(request *Transaction) (*TransactionInfo, error) {
	resp := TransactionInfo{}
	err := c.PostRequest(&resp, SubmitTranscationPath, request)
	return &resp, err
}

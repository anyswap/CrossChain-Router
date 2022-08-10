package aptos

import (
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

type RestClient struct {
	Url     string
	Timeout int
}

func (c *RestClient) GetRequest(result interface{}, url string, params map[string]string) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	for key, val := range params {
		url = strings.Replace(url, "{"+key+"}", val, -1)
	}
	return client.RPCGetRequest(result, c.Url+url, params, headers, c.Timeout)
}

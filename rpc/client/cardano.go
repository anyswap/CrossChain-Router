package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/anyswap/CrossChain-Router/v3/log"
)

// RequestBody request body
type CardanoRequestBody struct {
	Version string      `json:"jsonrpc"`
	Query   interface{} `json:"query"`
	ID      int         `json:"id"`
}

type CardanoJsonrpcResponse struct {
	Error  json.RawMessage `json:"errors,omitempty"`
	Result json.RawMessage `json:"data,omitempty"`
}

// RPCPostRequest rpc post request
func CardanoPostRequest(url string, req *Request, result interface{}) error {
	return CardanoPostRequestWithContext(httpCtx, url, req, result)
}

// RPCPostRequestWithContext rpc post request with context
func CardanoPostRequestWithContext(ctx context.Context, url string, req *Request, result interface{}) error {
	reqBody := &CardanoRequestBody{
		Version: "2.0",
		Query:   req.Params,
		ID:      req.ID,
	}
	resp, err := HTTPPostWithContext(ctx, url, reqBody, nil, nil, req.Timeout)
	if err != nil {
		log.Trace("post rpc error", "url", url, "request", req, "err", err)
		return err
	}
	err = CardanoGetResultFromJSONResponse(result, resp)
	if err != nil {
		log.Trace("post rpc error", "url", url, "request", req, "err", err)
	}
	return err
}

func CardanoGetResultFromJSONResponse(result interface{}, resp *http.Response) error {
	defer func() {
		_ = resp.Body.Close()
	}()
	const maxReadContentLength int64 = 1024 * 1024 * 10 // 10M
	body, err := ioutil.ReadAll(io.LimitReader(resp.Body, maxReadContentLength))
	if err != nil {
		return fmt.Errorf("read body error: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("wrong response status %v. message: %v", resp.StatusCode, string(body))
	}
	if len(body) == 0 {
		return fmt.Errorf("empty response body")
	}

	var jsonResp CardanoJsonrpcResponse
	err = json.Unmarshal(body, &jsonResp)
	if err != nil {
		return fmt.Errorf("unmarshal body error, body is \"%v\" err=\"%w\"", string(body), err)
	}
	if jsonResp.Error != nil {
		return fmt.Errorf("return error: %v", jsonResp.Error)
	}
	err = json.Unmarshal(jsonResp.Result, &result)
	if err != nil {
		return fmt.Errorf("unmarshal result error: %w", err)
	}
	return nil
}

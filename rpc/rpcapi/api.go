package rpcapi

import (
	"net/http"

	"github.com/anyswap/CrossChain-Router/internal/swapapi"
	"github.com/anyswap/CrossChain-Router/params"
)

// RouterSwapAPI rpc api handler
type RouterSwapAPI struct{}

// RPCNullArgs null args
type RPCNullArgs struct{}

// RouterSwapKeyArgs args
type RouterSwapKeyArgs struct {
	ChainID  string `json:"chainid"`
	TxID     string `json:"txid"`
	LogIndex string `json:"logindex"`
}

// GetVersionInfo api
func (s *RouterSwapAPI) GetVersionInfo(r *http.Request, args *RPCNullArgs, result *string) error {
	version := params.VersionWithMeta
	*result = version
	return nil
}

// GetIdentifier api
func (s *RouterSwapAPI) GetIdentifier(r *http.Request, args *RPCNullArgs, result *string) error {
	identifier := params.GetIdentifier()
	*result = identifier
	return nil
}

// RegisterRouterSwap api
func (s *RouterSwapAPI) RegisterRouterSwap(r *http.Request, args *RouterSwapKeyArgs, result *swapapi.MapIntResult) error {
	res, err := swapapi.RegisterRouterSwap(args.ChainID, args.TxID, args.LogIndex)
	if err == nil && res != nil {
		*result = *res
	}
	return err
}

// GetRouterSwap api
func (s *RouterSwapAPI) GetRouterSwap(r *http.Request, args *RouterSwapKeyArgs, result *swapapi.SwapInfo) error {
	res, err := swapapi.GetRouterSwap(args.ChainID, args.TxID, args.LogIndex)
	if err == nil && res != nil {
		*result = *res
	}
	return err
}

// RouterGetSwapHistoryArgs args
type RouterGetSwapHistoryArgs struct {
	ChainID string `json:"chainid"`
	Address string `json:"address"`
	Offset  int    `json:"offset"`
	Limit   int    `json:"limit"`
}

// GetRouterSwapHistory api
func (s *RouterSwapAPI) GetRouterSwapHistory(r *http.Request, args *RouterGetSwapHistoryArgs, result *[]*swapapi.SwapInfo) error {
	res, err := swapapi.GetRouterSwapHistory(args.ChainID, args.Address, args.Offset, args.Limit)
	if err == nil && res != nil {
		*result = res
	}
	return err
}

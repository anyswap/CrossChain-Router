package grpc

import (
	"context"
	"reflect"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	"github.com/pkg/errors"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var protoCodec = encoding.GetCodec(proto.Name)

// NewClientContext returns new context
func NewClientContext(clientCtx client.Context) ClientContext {
	return ClientContext{clientCtx}
}

// ClientContext exposes the functionality of SDK context in a way where we may intercept GRPC-related method (Invoke)
// to provide better implementation
type ClientContext struct {
	clientCtx client.Context
}

// ChainID returns chain ID
func (c ClientContext) ChainID() string {
	return c.clientCtx.ChainID
}

// WithChainID returns a copy of the context with an updated chain ID
func (c ClientContext) WithChainID(chainID string) ClientContext {
	c.clientCtx = c.clientCtx.WithChainID(chainID)
	return c
}

// WithClient returns a copy of the context with an updated RPC client
// instance
func (c ClientContext) WithClient(client rpcclient.Client) ClientContext {
	c.clientCtx = c.clientCtx.WithClient(client)
	return c
}

// WithBroadcastMode returns a copy of the context with an updated broadcast
// mode.
func (c ClientContext) WithBroadcastMode(mode string) ClientContext {
	c.clientCtx = c.clientCtx.WithBroadcastMode(mode)
	return c
}

// TxConfig returns TxConfig of SDK context
func (c ClientContext) TxConfig() client.TxConfig {
	return c.clientCtx.TxConfig
}

// WithFromName returns a copy of the context with an updated from account name
func (c ClientContext) WithFromName(name string) ClientContext {
	c.clientCtx = c.clientCtx.WithFromName(name)
	return c
}

// WithFromAddress returns a copy of the context with an updated from account address
func (c ClientContext) WithFromAddress(addr sdk.AccAddress) ClientContext {
	c.clientCtx = c.clientCtx.WithFromAddress(addr)
	return c
}

// NewStream implements the grpc ClientConn.NewStream method
func (c ClientContext) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("streaming rpc not supported")
}

// FeeGranterAddress returns the fee granter address from the context
func (c ClientContext) FeeGranterAddress() sdk.AccAddress {
	return c.clientCtx.GetFeeGranterAddress()
}

// FromName returns the key name for the current context.
func (c ClientContext) FromName() string {
	return c.clientCtx.GetFromName()
}

// FromAddress returns the from address from the context's name.
func (c ClientContext) FromAddress() sdk.AccAddress {
	return c.clientCtx.GetFromAddress()
}

// BroadcastMode returns configured tx broadcast mode
func (c ClientContext) BroadcastMode() string {
	return c.clientCtx.BroadcastMode
}

// Client returns RPC client
func (c ClientContext) Client() rpcclient.Client {
	return c.clientCtx.Client
}

// InterfaceRegistry returns interface registry of SDK context
func (c ClientContext) InterfaceRegistry() codectypes.InterfaceRegistry {
	return c.clientCtx.InterfaceRegistry
}

// Keyring returns keyring
func (c ClientContext) Keyring() keyring.Keyring {
	return c.clientCtx.Keyring
}

// WithKeyring returns a copy of the context with an updated keyring
func (c ClientContext) WithKeyring(k keyring.Keyring) ClientContext {
	c.clientCtx = c.clientCtx.WithKeyring(k)
	return c
}

// Invoke invokes GRPC method
func (c ClientContext) Invoke(ctx context.Context, method string, req, reply interface{}, opts ...grpc.CallOption) (err error) {
	if reflect.ValueOf(req).IsNil() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "request cannot be nil")
	}

	reqBz, err := protoCodec.Marshal(req)
	if err != nil {
		return err
	}

	// parse height header
	md, _ := metadata.FromOutgoingContext(ctx)
	height := c.clientCtx.Height
	if heights := md.Get(grpctypes.GRPCBlockHeightHeader); len(heights) > 0 {
		var err error
		height, err = strconv.ParseInt(heights[0], 10, 64)
		if err != nil {
			return err
		}
		if height < 0 {
			return sdkerrors.Wrapf(
				sdkerrors.ErrInvalidRequest,
				"client.Context.Invoke: height (%d) from %q must be >= 0", height, grpctypes.GRPCBlockHeightHeader)
		}
	}

	abciReq := abci.RequestQuery{
		Path:   method,
		Data:   reqBz,
		Height: height,
	}

	res, err := c.queryABCI(ctx, abciReq)
	if err != nil {
		return err
	}

	err = protoCodec.Unmarshal(res.Value, reply)
	if err != nil {
		return err
	}

	// Create header metadata. For now the headers contain:
	// - block height
	// We then parse all the call options, if the call option is a
	// HeaderCallOption, then we manually set the value of that header to the
	// metadata.
	md = metadata.Pairs(grpctypes.GRPCBlockHeightHeader, strconv.FormatInt(res.Height, 10))
	for _, callOpt := range opts {
		header, ok := callOpt.(grpc.HeaderCallOption)
		if !ok {
			continue
		}

		*header.HeaderAddr = md
	}

	if c.clientCtx.InterfaceRegistry != nil {
		return codectypes.UnpackInterfaces(reply, c.clientCtx.InterfaceRegistry)
	}

	return nil
}

func (c ClientContext) queryABCI(ctx context.Context, req abci.RequestQuery) (abci.ResponseQuery, error) {
	node, err := c.clientCtx.GetNode()
	if err != nil {
		return abci.ResponseQuery{}, err
	}

	opts := rpcclient.ABCIQueryOptions{
		Height: req.Height,
		Prove:  req.Prove,
	}

	result, err := node.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return abci.ResponseQuery{}, err
	}

	if !result.Response.IsOK() {
		return abci.ResponseQuery{}, sdkErrorToGRPCError(result.Response)
	}

	return result.Response, nil
}

func sdkErrorToGRPCError(resp abci.ResponseQuery) error {
	switch resp.Code {
	case sdkerrors.ErrInvalidRequest.ABCICode():
		return status.Error(codes.InvalidArgument, resp.Log)
	case sdkerrors.ErrUnauthorized.ABCICode():
		return status.Error(codes.Unauthenticated, resp.Log)
	case sdkerrors.ErrKeyNotFound.ABCICode():
		return status.Error(codes.NotFound, resp.Log)
	default:
		return status.Error(codes.Unknown, resp.Log)
	}
}

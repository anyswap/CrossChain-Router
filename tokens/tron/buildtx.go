package tron

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second
)

// BuildRawTransaction build raw tx
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.Input != nil {
		return nil, fmt.Errorf("forbid build raw swap tx with input data")
	}
	if args.From == "" {
		return nil, fmt.Errorf("forbid empty sender")
	}
	routerMPC, err := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}

	switch args.SwapType {
	case tokens.ERC20SwapType:
		err = b.buildERC20SwapTxInput(args)
	case tokens.NFTSwapType:
		err = b.buildNFTSwapTxInput(args)
	case tokens.AnyCallSwapType:
		err = b.buildAnyCallSwapTxInput(args)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	if err != nil {
		return nil, err
	}

	return b.buildTx(args)
}

var SwapinFeeLimit int32 = 300000000 // 300 TRX

func (b *Bridge) buildTx(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	var (
		to    = args.To
		extra = getOrInitTronExtra(args)
	)

	if extra.RawTx != "" {
		var bz []byte
		bz, err = hex.DecodeString(extra.RawTx)
		if err != nil {
			return nil, err
		}
		var raw core.Transaction
		err = proto.Unmarshal(bz, &raw)
		if err != nil {
			return nil, err
		}
		return &raw, nil
	}

	rawTx, err = b.BuildTriggerConstantContractTx(args.From, args.To, extra.Selector, extra.Params, SwapinFeeLimit)

	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "to", to, "bind", args.Bind,
		"replaceNum", args.GetReplaceNum(),
	}
	switch {
	case args.ERC20SwapInfo != nil:
		ctx = append(ctx,
			"originValue", args.OriginValue,
			"swapValue", args.SwapValue,
			"tokenID", args.ERC20SwapInfo.TokenID)
	case args.NFTSwapInfo != nil:
		ctx = append(ctx,
			"tokenID", args.NFTSwapInfo.TokenID,
			"ids", args.NFTSwapInfo.IDs,
			"amounts", args.NFTSwapInfo.Amounts,
			"batch", args.NFTSwapInfo.Batch)
	}
	if err != nil {
		ctx = append(ctx, "err", err)
	}
	log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()), ctx...)
	if err != nil {
		return nil, err
	}
	txmsg, err := proto.Marshal(rawTx.(*core.Transaction))
	if err != nil {
		return nil, err
	}
	extra.RawTx = fmt.Sprintf("%X", txmsg)

	return rawTx, nil
}

func getOrInitTronExtra(args *tokens.BuildTxArgs) *tokens.TronExtraArgs {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{TronExtra: &tokens.TronExtraArgs{}}
	} else if args.Extra.TronExtra == nil {
		args.Extra.TronExtra = &tokens.TronExtraArgs{}
	}
	return args.Extra.TronExtra
}

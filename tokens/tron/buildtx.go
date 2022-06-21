package tron

import (
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	proto "github.com/golang/protobuf/proto"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second

	latestGasPrice *big.Int
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
		to = args.To
		//input     = *args.Input
		extra     = args.Extra.EthExtra
		gasLimit  = *extra.Gas
		gasPrice  = extra.GasPrice
		gasTipCap = extra.GasTipCap
		gasFeeCap = extra.GasFeeCap
	)

	rawTx, err = b.BuildTriggerConstantContractTx(args.From, args.To, args.Extra.TronExtra.Selector, args.Extra.TronExtra.Params, SwapinFeeLimit)

	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "to", to, "bind", args.Bind,
		"gasLimit", gasLimit, "replaceNum", args.GetReplaceNum(),
	}
	if gasTipCap != nil || gasFeeCap != nil {
		ctx = append(ctx, "gasTipCap", gasTipCap, "gasFeeCap", gasFeeCap)
	} else {
		ctx = append(ctx, "gasPrice", gasPrice)
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
	log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()), ctx...)
	txmsg, err := proto.Marshal(rawTx.(*core.Transaction))
	if err != nil {
		return nil, err
	}
	args.Extra.TronExtra.RawTx = fmt.Sprintf("%X", txmsg)

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

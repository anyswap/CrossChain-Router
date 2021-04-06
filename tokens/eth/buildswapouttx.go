package eth

import (
	"errors"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/common"
	"github.com/anyswap/CrossChain-Router/common/hexutil"
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/params"
	"github.com/anyswap/CrossChain-Router/router"
	"github.com/anyswap/CrossChain-Router/tokens"
	"github.com/anyswap/CrossChain-Router/tokens/eth/abicoder"
)

// router contract's func hashs
var (
	defSwapDeadlineOffset = int64(36000)

	// anySwapIn(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInFuncHash = common.FromHex("0x825bb13c")
	// anySwapInUnderlying(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInUnderlyingFuncHash = common.FromHex("0x3f88de89")
	// anySwapInAuto(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInAutoFuncHash = common.FromHex("0x0175b1c4")
	// anySwapInExactTokensForTokens(bytes32 txs, uint amountIn, uint amountOutMin, address[] path, address to, uint deadline, uint fromChainID)
	AnySwapInExactTokensForTokensFuncHash = common.FromHex("0x2fc1e728")
	// anySwapInExactTokensForNative(bytes32 txs, uint amountIn, uint amountOutMin, address[] path, address to, uint deadline, uint fromChainID)
	AnySwapInExactTokensForNativeFuncHash = common.FromHex("0x52a397d5")
)

func (b *Bridge) buildRouterSwapTxInput(args *tokens.BuildTxArgs) (err error) {
	if !params.IsRouterSwap() || b.ChainConfig.RouterContract == "" {
		return tokens.ErrRouterSwapNotSupport
	}
	if len(args.Path) > 0 && args.AmountOutMin != nil {
		return b.buildRouterSwapTradeTxInput(args)
	}
	return b.buildRouterSwapoutTxInput(args)
}

func (b *Bridge) buildRouterSwapoutTxInput(args *tokens.BuildTxArgs) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args)
	if err != nil {
		return err
	}

	var funcHash []byte
	if args.ForUnderlying {
		funcHash = AnySwapInUnderlyingFuncHash
	} else {
		funcHash = AnySwapInAutoFuncHash // old:AnySwapInFuncHash
	}

	multichainToken := router.GetCachedMultichainToken(args.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", args.TokenID, "chainID", args.ToChainID)
		return tokens.ErrMissTokenConfig
	}

	input := abicoder.PackDataWithFuncHash(funcHash,
		common.HexToHash(args.SwapID),
		common.HexToAddress(multichainToken),
		receiver,
		amount,
		args.FromChainID,
	)
	args.Input = (*hexutil.Bytes)(&input)  // input
	args.To = b.ChainConfig.RouterContract // to
	args.SwapValue = amount                // swapValue

	return nil
}

func (b *Bridge) buildRouterSwapTradeTxInput(args *tokens.BuildTxArgs) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args)
	if err != nil {
		return err
	}

	var funcHash []byte
	if args.ForNative {
		funcHash = AnySwapInExactTokensForNativeFuncHash
	} else {
		funcHash = AnySwapInExactTokensForTokensFuncHash
	}

	swapDeadlineOffset := b.ChainConfig.SwapDeadlineOffset
	if swapDeadlineOffset == 0 {
		swapDeadlineOffset = defSwapDeadlineOffset
	}
	deadline := time.Now().Unix() + swapDeadlineOffset

	input := abicoder.PackDataWithFuncHash(funcHash,
		common.HexToHash(args.SwapID),
		amount,
		args.AmountOutMin,
		toAddresses(args.Path),
		receiver,
		deadline,
		args.FromChainID,
	)
	args.Input = (*hexutil.Bytes)(&input)  // input
	args.To = b.ChainConfig.RouterContract // to
	args.SwapValue = amount                // swapValue

	return nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs) (receiver common.Address, amount *big.Int, err error) {
	receiver = common.HexToAddress(args.Bind)
	if receiver == (common.Address{}) || !common.IsHexAddress(args.Bind) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("can not swapout to empty or invalid receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	fromTokenCfg := fromBridge.GetTokenConfig(args.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", args.Token)
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	amount = tokens.CalcSwapValue(fromTokenCfg, args.OriginValue)
	return receiver, amount, err
}

func toAddresses(path []string) []common.Address {
	addresses := make([]common.Address, len(path))
	for i, addr := range path {
		addresses[i] = common.HexToAddress(addr)
	}
	return addresses
}

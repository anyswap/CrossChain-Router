package eth

import (
	"errors"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
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
	if args.RouterSwapInfo == nil || args.TokenID == "" {
		return errors.New("build router swaptx without tokenID")
	}
	multichainToken := router.GetCachedMultichainToken(args.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", args.TokenID, "chainID", args.ToChainID)
		return tokens.ErrMissTokenConfig
	}

	if len(args.Path) > 0 && args.AmountOutMin != nil {
		return b.buildRouterSwapTradeTxInput(args, multichainToken)
	}
	return b.buildRouterSwapoutTxInput(args, multichainToken)
}

func (b *Bridge) buildRouterSwapoutTxInput(args *tokens.BuildTxArgs, multichainToken string) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return err
	}

	var funcHash []byte
	if args.ForUnderlying {
		funcHash = AnySwapInUnderlyingFuncHash
	} else {
		funcHash = AnySwapInAutoFuncHash // old:AnySwapInFuncHash
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

func (b *Bridge) buildRouterSwapTradeTxInput(args *tokens.BuildTxArgs, multichainToken string) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return err
	}

	var funcHash []byte
	if args.ForNative {
		funcHash = AnySwapInExactTokensForNativeFuncHash
	} else {
		funcHash = AnySwapInExactTokensForTokensFuncHash
	}

	input := abicoder.PackDataWithFuncHash(funcHash,
		common.HexToHash(args.SwapID),
		amount,
		args.AmountOutMin,
		toAddresses(args.Path),
		receiver,
		calcSwapDeadline(args),
		args.FromChainID,
	)
	args.Input = (*hexutil.Bytes)(&input)  // input
	args.To = b.ChainConfig.RouterContract // to
	args.SwapValue = amount                // swapValue

	return nil
}

func calcSwapDeadline(args *tokens.BuildTxArgs) int64 {
	var deadline int64
	if args.Extra != nil && args.Extra.EthExtra != nil {
		deadline = args.Extra.EthExtra.Deadline
	} else if serverCfg := params.GetRouterServerConfig(); serverCfg != nil {
		offset := serverCfg.SwapDeadlineOffset
		if offset == 0 {
			offset = defSwapDeadlineOffset
		}
		deadline = time.Now().Unix() + offset
		getOrInitEthExtra(args).Deadline = deadline
	}
	return deadline
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver common.Address, amount *big.Int, err error) {
	receiver = common.HexToAddress(args.Bind)
	if receiver == (common.Address{}) || !common.IsHexAddress(args.Bind) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("can not swapout to empty or invalid receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	if fromBridge == nil {
		return receiver, amount, tokens.ErrNoBridgeForChainID
	}
	fromTokenCfg := fromBridge.GetTokenConfig(args.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", args.Token)
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	swapFeeOn := router.SwapFeeOnFlags[b.ChainConfig.ChainID]
	amount = tokens.CalcSwapValue(fromTokenCfg, toTokenCfg, args.OriginValue, swapFeeOn)
	return receiver, amount, err
}

func toAddresses(path []string) []common.Address {
	addresses := make([]common.Address, len(path))
	for i, addr := range path {
		addresses[i] = common.HexToAddress(addr)
	}
	return addresses
}

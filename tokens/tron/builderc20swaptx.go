package tron

import (
	"errors"
	"math/big"

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
	ForceAnySwapInAutoTokenVersion             = uint64(10001)
	ForceAnySwapInTokenVersion                 = uint64(10002)
	ForceAnySwapInUnderlyingTokenVersion       = uint64(10003)
	ForceAnySwapInNativeTokenVersion           = uint64(10004)
	ForceAnySwapInAndCallTokenVersion          = uint64(10005)
	ForceAnySwapInUnerlyingAndCallTokenVersion = uint64(10006)

	// anySwapIn(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInFuncHash = "anySwapIn(bytes32,address,address,uint256,uint256)"
	// anySwapInUnderlying(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInUnderlyingFuncHash = "anySwapInUnderlying(bytes32,address,address,uint256,uint256)"
	// anySwapInNative(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInNativeFuncHash = "anySwapInNative(bytes32,address,address,uint256,uint256)"
	// anySwapInAuto(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInAutoFuncHash = "anySwapInAuto(bytes32,address,address,uint256,uint256)"
	// anySwapInAndExec(bytes32 txs, address token, address to, uint amount, uint fromChainID, address anycallProxy, bytes calldata data)
	AnySwapInAndExecFuncHash = "anySwapInAndExec(bytes32,address,address,uint256,uint256,address,bytes)"
	// anySwapInUnderlyingAndExec(bytes32 txs, address token, address to, uint amount, uint fromChainID, address anycallProxy, bytes calldata data)
	AnySwapInUnderlyingAndExecFuncHash = "anySwapInUnderlyingAndExec(bytes32,address,address,uint256,uint256,address,bytes)"
)

// GetSwapInFuncHash get swapin func hash
func GetSwapInFuncHash(tokenCfg *tokens.TokenConfig) string {
	switch tokenCfg.ContractVersion {
	case ForceAnySwapInAutoTokenVersion:
		return AnySwapInAutoFuncHash
	case ForceAnySwapInTokenVersion, ForceAnySwapInAndCallTokenVersion:
		return AnySwapInFuncHash
	case ForceAnySwapInUnderlyingTokenVersion, ForceAnySwapInUnerlyingAndCallTokenVersion:
		return AnySwapInUnderlyingFuncHash
	case ForceAnySwapInNativeTokenVersion:
		return AnySwapInNativeFuncHash
	case 0:
		if common.HexToAddress(tokenCfg.GetUnderlying()) == (common.Address{}) {
			// without underlying
			return AnySwapInFuncHash
		}
	default:
		if common.HexToAddress(tokenCfg.GetUnderlying()) == (common.Address{}) &&
			!params.IsForceAnySwapInAuto() {
			// without underlying, and not force swapinAuto
			return AnySwapInFuncHash
		}
	}
	return AnySwapInAutoFuncHash
}

// GetSwapInAndExecFuncHash get swapin and call func hash
func GetSwapInAndExecFuncHash(tokenCfg *tokens.TokenConfig) string {
	switch tokenCfg.ContractVersion {
	case ForceAnySwapInAndCallTokenVersion:
		return AnySwapInAndExecFuncHash
	case ForceAnySwapInUnerlyingAndCallTokenVersion:
		return AnySwapInUnderlyingAndExecFuncHash
	default:
		if common.HexToAddress(tokenCfg.GetUnderlying()) == (common.Address{}) {
			return AnySwapInAndExecFuncHash
		}
	}
	return AnySwapInUnderlyingAndExecFuncHash
}

func (b *Bridge) buildERC20SwapTxInput(args *tokens.BuildTxArgs) (err error) {
	if args.ERC20SwapInfo == nil || args.ERC20SwapInfo.TokenID == "" {
		return errors.New("build router swaptx without tokenID")
	}
	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return tokens.ErrMissTokenConfig
	}

	if erc20SwapInfo.CallProxy != "" {
		return b.buildSwapAndExecTxInput(args, multichainToken)
	}
	return b.buildERC20SwapinTxInput(args, multichainToken)
}

func (b *Bridge) buildSwapAndExecTxInput(args *tokens.BuildTxArgs, multichainToken string) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return err
	}

	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}

	erc20SwapInfo := args.ERC20SwapInfo

	args.Selector = GetSwapInAndExecFuncHash(toTokenCfg)

	input := abicoder.PackData(
		common.HexToHash(args.SwapID),
		common.HexToAddress(convertToEthAddress(multichainToken)),
		receiver,
		amount,
		args.FromChainID,
		common.HexToAddress(erc20SwapInfo.CallProxy),
		erc20SwapInfo.CallData,
	)
	args.Input = (*hexutil.Bytes)(&input) // input
	routerContract := b.GetRouterContract(multichainToken)
	args.To = routerContract // to
	args.SwapValue = amount  // swapValue

	return nil
}

func (b *Bridge) buildERC20SwapinTxInput(args *tokens.BuildTxArgs, multichainToken string) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return err
	}

	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}

	args.Selector = GetSwapInFuncHash(toTokenCfg)

	input := abicoder.PackData(
		common.HexToHash(args.SwapID),
		common.HexToAddress(convertToEthAddress(multichainToken)),
		receiver,
		amount,
		args.FromChainID,
	)
	args.Input = (*hexutil.Bytes)(&input) // input
	routerContract := b.GetRouterContract(multichainToken)
	args.To = routerContract // to
	args.SwapValue = amount  // swapValue

	return nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver common.Address, amount *big.Int, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	ethAddress := args.Bind
	if !common.IsHexAddress(ethAddress) {
		ethAddress, err = tronToEth(args.Bind)
		if err != nil {
			log.Warn("swapout to wrong receiver", "receiver", args.Bind, "err", err)
			return receiver, amount, err
		}
	}
	receiver = common.HexToAddress(ethAddress)
	if receiver == (common.Address{}) {
		log.Warn("swapout to empty receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("can not swapout to empty receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	if fromBridge == nil {
		return receiver, amount, tokens.ErrNoBridgeForChainID
	}
	fromTokenCfg := fromBridge.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", erc20SwapInfo.Token)
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	amount = tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	return receiver, amount, err
}

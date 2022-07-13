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

	ForceAnySwapInAutoTokenVersion             = uint64(10001)
	ForceAnySwapInTokenVersion                 = uint64(10002)
	ForceAnySwapInUnderlyingTokenVersion       = uint64(10003)
	ForceAnySwapInNativeTokenVersion           = uint64(10004)
	ForceAnySwapInAndCallTokenVersion          = uint64(10005)
	ForceAnySwapInUnerlyingAndCallTokenVersion = uint64(10006)
	MintBurnWrapperTokenVersion                = uint64(20000)

	// anySwapIn(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInFuncHash = common.FromHex("0x825bb13c")
	// anySwapInUnderlying(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInUnderlyingFuncHash = common.FromHex("0x3f88de89")
	// anySwapInNative(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInNativeFuncHash = common.FromHex("0x21974f28")
	// anySwapInAuto(bytes32 txs, address token, address to, uint amount, uint fromChainID)
	AnySwapInAutoFuncHash = common.FromHex("0x0175b1c4")
	// anySwapInExactTokensForTokens(bytes32 txs, uint amountIn, uint amountOutMin, address[] path, address to, uint deadline, uint fromChainID)
	AnySwapInExactTokensForTokensFuncHash = common.FromHex("0x2fc1e728")
	// anySwapInExactTokensForNative(bytes32 txs, uint amountIn, uint amountOutMin, address[] path, address to, uint deadline, uint fromChainID)
	AnySwapInExactTokensForNativeFuncHash = common.FromHex("0x52a397d5")

	// ----------------------- The following is for router(v7) + anycall --------------------

	// anySwapIn(string swapID, address token, address to, uint amount, uint fromChainID)
	AnySwapInFuncHash2 = common.FromHex("0x8619c43d")
	// anySwapInUnderlying(string swapID, address token, address to, uint amount, uint fromChainID)
	AnySwapInUnderlyingFuncHash2 = common.FromHex("0x6e1f1a50")
	// anySwapInNative(string swapID, address token, address to, uint amount, uint fromChainID)
	AnySwapInNativeFuncHash2 = common.FromHex("0x174820bd")
	// anySwapInAuto(string swapID, address token, address to, uint amount, uint fromChainID)
	AnySwapInAutoFuncHash2 = common.FromHex("0xa5e4d7a1")
	// anySwapInAndExec(string swapID, address token, address to, uint amount, uint fromChainID, address anycallProxy, bytes calldata data)
	AnySwapInAndExecFuncHash = common.FromHex("0x988046dc")
	// anySwapInUnderlyingAndExec(string swapID, address token, address to, uint amount, uint fromChainID, address anycallProxy, bytes calldata data)
	AnySwapInUnderlyingAndExecFuncHash = common.FromHex("0x4c41e752")
)

// GetSwapInFuncHash1 get swapin func hash
func GetSwapInFuncHash1(tokenCfg *tokens.TokenConfig, forUnderlying bool) []byte {
	if forUnderlying {
		return AnySwapInUnderlyingFuncHash
	}

	switch tokenCfg.ContractVersion {
	case ForceAnySwapInAutoTokenVersion:
		return AnySwapInAutoFuncHash
	case ForceAnySwapInTokenVersion, ForceAnySwapInAndCallTokenVersion, MintBurnWrapperTokenVersion:
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

// GetSwapInFuncHash2 get swapin func hash
func GetSwapInFuncHash2(tokenCfg *tokens.TokenConfig, forUnderlying bool) []byte {
	if forUnderlying {
		return AnySwapInUnderlyingFuncHash2
	}

	switch tokenCfg.ContractVersion {
	case ForceAnySwapInAutoTokenVersion:
		return AnySwapInAutoFuncHash2
	case ForceAnySwapInTokenVersion, ForceAnySwapInAndCallTokenVersion, MintBurnWrapperTokenVersion:
		return AnySwapInFuncHash2
	case ForceAnySwapInUnderlyingTokenVersion, ForceAnySwapInUnerlyingAndCallTokenVersion:
		return AnySwapInUnderlyingFuncHash2
	case ForceAnySwapInNativeTokenVersion:
		return AnySwapInNativeFuncHash2
	case 0:
		if common.HexToAddress(tokenCfg.GetUnderlying()) == (common.Address{}) {
			// without underlying
			return AnySwapInFuncHash2
		}
	default:
		if common.HexToAddress(tokenCfg.GetUnderlying()) == (common.Address{}) &&
			!params.IsForceAnySwapInAuto() {
			// without underlying, and not force swapinAuto
			return AnySwapInFuncHash2
		}
	}
	return AnySwapInAutoFuncHash2
}

// GetSwapInAndExecFuncHash get swapin and call func hash
func GetSwapInAndExecFuncHash(tokenCfg *tokens.TokenConfig) []byte {
	switch tokenCfg.ContractVersion {
	case ForceAnySwapInAndCallTokenVersion, MintBurnWrapperTokenVersion:
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
	if len(erc20SwapInfo.Path) > 0 {
		return b.buildERC20SwapTradeTxInput(args, multichainToken)
	}
	return b.buildERC20SwapoutTxInput(args, multichainToken)
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

	funcHash := GetSwapInAndExecFuncHash(toTokenCfg)

	input := abicoder.PackDataWithFuncHash(funcHash,
		args.GetUniqueSwapIdentifier(),
		common.HexToAddress(multichainToken),
		receiver,
		amount,
		args.FromChainID,
		common.HexToAddress(erc20SwapInfo.CallProxy),
		erc20SwapInfo.CallData,
	)
	args.Input = (*hexutil.Bytes)(&input)          // input
	args.To = b.GetRouterContract(multichainToken) // to
	args.SwapValue = amount                        // swapValue

	return nil
}

func (b *Bridge) buildERC20SwapoutTxInput(args *tokens.BuildTxArgs, multichainToken string) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return err
	}

	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}

	var funcHash, input []byte

	switch params.GetSwapSubType() {
	case "v7":
		funcHash = GetSwapInFuncHash2(toTokenCfg, args.ERC20SwapInfo.ForUnderlying)
		input = abicoder.PackDataWithFuncHash(funcHash,
			args.GetUniqueSwapIdentifier(),
			common.HexToAddress(multichainToken),
			receiver,
			amount,
			args.FromChainID,
		)
	default:
		funcHash = GetSwapInFuncHash1(toTokenCfg, args.ERC20SwapInfo.ForUnderlying)
		input = abicoder.PackDataWithFuncHash(funcHash,
			common.HexToHash(args.SwapID),
			common.HexToAddress(multichainToken),
			receiver,
			amount,
			args.FromChainID,
		)
	}

	args.Input = (*hexutil.Bytes)(&input)          // input
	args.To = b.GetRouterContract(multichainToken) // to
	args.SwapValue = amount                        // swapValue

	return nil
}

func (b *Bridge) buildERC20SwapTradeTxInput(args *tokens.BuildTxArgs, multichainToken string) (err error) {
	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return err
	}
	erc20SwapInfo := args.ERC20SwapInfo

	var funcHash []byte
	if erc20SwapInfo.ForNative {
		funcHash = AnySwapInExactTokensForNativeFuncHash
	} else {
		funcHash = AnySwapInExactTokensForTokensFuncHash
	}

	input := abicoder.PackDataWithFuncHash(funcHash,
		common.HexToHash(args.SwapID),
		amount,
		erc20SwapInfo.AmountOutMin,
		toAddresses(erc20SwapInfo.Path),
		receiver,
		calcSwapDeadline(args),
		args.FromChainID,
	)
	args.Input = (*hexutil.Bytes)(&input)          // input
	args.To = b.GetRouterContract(multichainToken) // to
	args.SwapValue = amount                        // swapValue

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
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = common.HexToAddress(args.Bind)
	if receiver == (common.Address{}) || !common.IsHexAddress(args.Bind) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("can not swapout to empty or invalid receiver")
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

func toAddresses(path []string) []common.Address {
	addresses := make([]common.Address, len(path))
	for i, addr := range path {
		addresses[i] = common.HexToAddress(addr)
	}
	return addresses
}

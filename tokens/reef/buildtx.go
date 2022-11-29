package reef

import (
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
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
	case tokens.ERC20SwapType, tokens.ERC20SwapTypeMixPool:
		err = b.BuildERC20SwapTxInput(args)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	if err != nil {
		return nil, err
	}

	evmAddr, err := b.QueryEvmAddress(routerMPC)
	if err != nil {
		return nil, err
	}

	signInfo, err := GetSignInfo(common.Bytes2Hex(*args.Input), evmAddr.Hex(), routerMPC, args.To)
	if err != nil {
		return nil, err
	}

	err = b.setDefaults(args, signInfo)
	if err != nil {
		return nil, err
	}

	return b.buildTx(args, evmAddr.Hex())
}

func (b *Bridge) buildTx(args *tokens.BuildTxArgs, evmAddr string) (rawTx interface{}, err error) {
	var (
		to       = args.To
		value    = args.Value
		input    = *args.Input
		extra    = args.Extra.EthExtra
		gasLimit = *extra.Gas
		gasPrice = extra.GasPrice.Uint64()

		isDynamicFeeTx = params.IsDynamicFeeTxEnabled(b.ChainConfig.ChainID)
	)

	if params.IsSwapServer ||
		(params.GetRouterOracleConfig() != nil &&
			params.GetRouterOracleConfig().CheckGasTokenBalance) {
		minReserveFee := b.getMinReserveFee()
		if minReserveFee.Sign() > 0 {
			needValue := big.NewInt(0)
			if value != nil && value.Sign() > 0 {
				needValue.Add(needValue, value)
			}
			needValue.Add(needValue, minReserveFee)

			err = b.checkCoinBalance(args.From, needValue)
			if err != nil {
				log.Warn("not enough coin balance", "tx.value", value, "gasLimit", gasLimit, "gasPrice", gasPrice, "minReserveFee", minReserveFee, "needValue", needValue, "isDynamic", isDynamicFeeTx, "swapID", args.SwapID, "err", err)
				return nil, err
			}
		}
	}

	// assign nonce immediately before construct tx
	// esp. for parallel signing, this can prevent nonce hole
	if extra.Nonce == nil { // server logic
		return nil, fmt.Errorf("reef buildTx nonce is empty")
	}

	nonce := *extra.Nonce

	rawTx = &ReefTransaction{
		To:           &to,
		From:         &args.From,
		EvmAddress:   &evmAddr,
		Data:         &input,
		AccountNonce: extra.Nonce,
		Amount:       value,
		GasLimit:     &gasLimit,
		StorageGas:   &gasPrice,
		BlockHash:    args.Extra.BlockHash,
		BlockNumber:  args.Extra.Sequence,
	}

	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "evmAddr", evmAddr, "to", to, "bind", args.Bind, "nonce", nonce,
		"gasLimit", gasLimit, "replaceNum", args.GetReplaceNum(),
		"gasPrice", gasPrice,
	}
	switch {
	case args.ERC20SwapInfo != nil:
		ctx = append(ctx,
			"originValue", args.OriginValue,
			"swapValue", args.SwapValue,
			"tokenID", args.ERC20SwapInfo.TokenID)
	case args.NFTSwapInfo != nil:
		return nil, fmt.Errorf("reef buildTx NFTSwapInfo not support")
	}
	log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()), ctx...)

	return rawTx, nil
}

func getOrInitEthExtra(args *tokens.BuildTxArgs) *tokens.EthExtraArgs {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{EthExtra: &tokens.EthExtraArgs{}}
	} else if args.Extra.EthExtra == nil {
		args.Extra.EthExtra = &tokens.EthExtraArgs{}
	}
	return args.Extra.EthExtra
}

func (b *Bridge) setDefaults(args *tokens.BuildTxArgs, signInfo []string) (err error) {
	if args.Value == nil {
		args.Value = new(big.Int)
	}
	extra := getOrInitEthExtra(args)
	if extra.GasPrice == nil {
		extra.GasPrice, _ = new(big.Int).SetString(signInfo[1], 10)
	}
	if extra.Gas == nil {
		extra.Gas = new(uint64)
		*extra.Gas, err = strconv.ParseUint(signInfo[0], 10, 64)
		if err != nil {
			return err
		}
	}
	if extra.Nonce == nil {
		extra.Nonce = new(uint64)
		*extra.Nonce, err = strconv.ParseUint(signInfo[4], 10, 64)
		if err != nil {
			return err
		}
	}
	if args.Extra.BlockHash == nil {
		args.Extra.BlockHash = &signInfo[2]
	}

	if args.Extra.Sequence == nil {
		args.Extra.Sequence = new(uint64)
		*args.Extra.Sequence, err = strconv.ParseUint(signInfo[3], 10, 64)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Bridge) getMinReserveFee() *big.Int {
	config := params.GetRouterConfig()
	if config == nil {
		return big.NewInt(0)
	}
	minReserve := params.GetMinReserveFee(b.ChainConfig.ChainID)
	if minReserve == nil {
		minReserve = big.NewInt(3e18) // default 3 reef
	}
	return minReserve
}

func (b *Bridge) checkCoinBalance(sender string, needValue *big.Int) (err error) {
	var balance *big.Int
	for i := 0; i < retryRPCCount; i++ {
		balance, err = b.GetBalance(sender)
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err == nil && balance.Cmp(needValue) < 0 {
		return fmt.Errorf("not enough coin balance. %v < %v", balance, needValue)
	}
	if err != nil {
		log.Warn("get balance error", "sender", sender, "err", err)
	}
	return err
}

// 0105 8400
// 62c48aa955218081a6e168b8808d641fd9994ea226d9d572383d03bd1fdad747 // 公钥
// 01f29e45be8d0db72aa3ae5115960fc1f30274b7f95145294c2406943c90c0f66f0d8937f19b1fa43cbb5f66e7b22d244c6787fc07cefb95b62e2ff5f6c2f28a80 // 签名
// 0503b5020015006e0aa801aa5b971eceb1dad8d7cb9237a18617fd9102
// 825bb13c5f31dac7618ccf2df75e0f5c458603d7a3ee2acb48d977ee41da3e562d7a90f60000000000000000000000003a641961cefa97052ec7f283c408cab9682f540a00000000000000000000000064e55a52425993d2b059cb398ec860c0339bcd01000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000001691 // input
// 00000000000000000000000000000000fb8317000000000000000000

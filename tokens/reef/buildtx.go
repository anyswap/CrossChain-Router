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
	// evmAddress
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

	mpcEvmAddr := common.HexToAddress(routerMPC)
	mpcPublickey := router.GetMPCPublicKey(routerMPC)
	mpcReefAddr := PubkeyToReefAddress(mpcPublickey)

	// reefAddr, err := b.QueryReefAddress(routerMPC)
	// if err != nil {
	// 	return nil, err
	// }

	signInfo, err := GetSignInfo(common.Bytes2HexWithPrefix(*args.Input), mpcEvmAddr.Hex(), mpcReefAddr, args.To)
	if err != nil {
		return nil, err
	}

	err = b.setDefaults(args, signInfo, mpcReefAddr)
	if err != nil {
		return nil, err
	}

	return b.buildTx(args, mpcEvmAddr.Hex(), mpcReefAddr)
}

func (b *Bridge) buildTx(args *tokens.BuildTxArgs, mpcEvmAddr, mpcReefAddr string) (rawTx interface{}, err error) {
	var (
		to       = args.To
		value    = args.Value
		input    = *args.Input
		extra    = args.Extra
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

			err = b.checkCoinBalance(mpcReefAddr, needValue)
			if err != nil {
				log.Warn("not enough coin balance", "tx.value", value, "gasLimit", gasLimit, "gasPrice", gasPrice, "minReserveFee", minReserveFee, "needValue", needValue, "isDynamic", isDynamicFeeTx, "swapID", args.SwapID, "err", err)
				return nil, err
			}
		}
	}

	// assign nonce immediately before construct tx
	// esp. for parallel signing, this can prevent nonce hole
	if extra.Sequence == nil { // server logic
		return nil, fmt.Errorf("reef buildTx nonce is empty")
	}

	nonce := *extra.Sequence

	rawTx = &ReefTransaction{
		To:           &to,
		From:         &args.From,
		EvmAddress:   &mpcEvmAddr,
		ReefAddress:  &mpcReefAddr,
		Data:         &input,
		AccountNonce: extra.Sequence,
		Amount:       value,
		GasLimit:     &gasLimit,
		StorageGas:   &gasPrice,
		BlockHash:    args.Extra.BlockHash,
		BlockNumber:  args.Extra.Sequence,
	}

	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "evmAddr", mpcEvmAddr, "reefAddress", mpcReefAddr, "to", to, "bind", args.Bind, "nonce", nonce,
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

func (b *Bridge) setDefaults(args *tokens.BuildTxArgs, signInfo []string, mpcReefAddr string) (err error) {
	if args.Value == nil {
		args.Value = new(big.Int)
	}
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	extra := args.Extra
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
	if extra.Sequence == nil {
		extra.Sequence = new(uint64)
		*extra.Sequence, err = strconv.ParseUint(signInfo[4], 10, 64)
		if err != nil {
			return err
		}
		b.AdjustNonce(mpcReefAddr, *extra.Sequence)
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

func (b *Bridge) checkCoinBalance(reefAddr string, needValue *big.Int) (err error) {
	var balance *big.Int
	for i := 0; i < retryRPCCount; i++ {
		balance, err = b.GetBalance(reefAddr)
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err == nil && balance.Cmp(needValue) < 0 {
		return fmt.Errorf("not enough coin balance. %v < %v", balance, needValue)
	}
	if err != nil {
		log.Warn("get balance error", "sender", reefAddr, "err", err)
	}
	return err
}

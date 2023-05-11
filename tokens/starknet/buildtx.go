package starknet

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet/rpcv02"
	"github.com/dontpanicdao/caigo/types"
)

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
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return nil, tokens.ErrMissMPCPublicKey
	}

	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return nil, tokens.ErrMissTokenConfig
	}

	token := b.GetTokenConfig(multichainToken)
	if token == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	receiver := args.Bind
	if !b.IsValidAddress(receiver) {
		return nil, tokens.ErrTxWithWrongReceiver
	}
	swapInArgs, err := b.buildSwapInArgs(args.SwapID, args.GetTokenID(), receiver, args.FromChainID, args.SwapValue)
	if err != nil {
		return nil, err
	}

	return b.buildTx(args, b.GetRouterContract(multichainToken), swapInArgs)
}

func (b *Bridge) buildSwapInArgs(txHash string, tokenID string, to string, fromChainID *big.Int, amountBN *big.Int) (*SwapIn, error) {
	amount, err := common.GetUint64FromStr(amountBN.String())
	if err != nil {
		return nil, err
	}
	calldata := SwapIn{
		Tx:          txHash,
		Token:       tokenID,
		To:          to,
		FromChainId: fromChainID.String(),
		Amount:      amount,
	}
	return &calldata, nil
}

func (b *Bridge) buildTx(args *tokens.BuildTxArgs, routerContractAddress string, swapInArgs *SwapIn) (rawTx interface{}, err error) {
	call := FunctionCall{
		ContractAddress:    HexToHash(routerContractAddress),
		EntryPointSelector: args.Selector,
		Calldata:           swapInArgs.getCalldata(),
	}

	estimate, err := b.sendEstimateTx(call)
	if err != nil {
		return nil, err
	}
	invokeNonce, err := b.GetPoolNonce(b.mpcAccount.Address, "")
	if err != nil {
		return nil, err
	}

	return FunctionCallWithDetails{
		Call:   call,
		MaxFee: estimate,
		Nonce:  new(big.Int).SetUint64(invokeNonce),
	}, nil
}

func (b *Bridge) sendEstimateTx(call FunctionCall) (*big.Int, error) {
	nonce, err := b.GetPoolNonce(b.defaultAccount.Address, "")
	if err != nil {
		return nil, err
	}
	details := &ExecuteDetails{
		Nonce:  new(big.Int).SetUint64(nonce),
		MaxFee: MAXFEE,
	}

	txHash, err := b.computeTxHash(call, details)
	if err != nil {
		return nil, err
	}

	r, s, err := b.defaultAccount.Sign(txHash)
	if err != nil {
		return nil, err
	}

	funcInvoke, err := b.prepFunctionInvoke([]FunctionCall{call}, details, r, s)
	if err != nil {
		return nil, err
	}

	estimate, err := b.EstimateFee(*funcInvoke)
	if err != nil {
		return nil, err
	}
	fee, ok := big.NewInt(0).SetString(string(estimate.OverallFee), 0)
	if !ok {
		return nil, tokens.ErrMatchFee
	}
	invokeMaxFee := fee.Mul(fee, big.NewInt(2))

	return invokeMaxFee, nil
}

func (b *Bridge) computeTxHash(call FunctionCall, details *ExecuteDetails) (txHash *big.Int, err error) {
	txHash, err = b.TransactionHash(call, details.MaxFee, details.Nonce)
	if err != nil {
		return nil, err
	}
	return txHash, nil
}

func (b *Bridge) prepFunctionInvoke(calls []FunctionCall, details *ExecuteDetails, r, s *big.Int) (*types.FunctionInvoke, error) {
	version, _ := big.NewInt(0).SetString(TxV1, 0)
	calldata := fmtCalldataStrings(calls)
	return &types.FunctionInvoke{
		MaxFee:    details.MaxFee,
		Version:   version,
		Signature: types.Signature{r, s},
		FunctionCall: types.FunctionCall{
			ContractAddress: types.HexToHash(b.defaultAccount.Address),
			Calldata:        calldata,
		},
		Nonce: details.Nonce,
	}, nil
}

func (b *Bridge) EstimateFee(call types.FunctionInvoke) (*types.FeeEstimate, error) {
	var signature []string
	for _, s := range call.Signature {
		signature = append(signature, fmt.Sprintf("0x%s", s.Text(16)))
	}
	c := rpcv02.BroadcastedInvokeV1Transaction{
		MaxFee:        call.MaxFee,
		Version:       rpcv02.TransactionV1,
		Signature:     signature,
		Nonce:         call.Nonce,
		Type:          TxTypeInvoke,
		Calldata:      call.FunctionCall.Calldata,
		SenderAddress: types.HexToHash(b.defaultAccount.Address),
	}
	return b.provider.EstimateFee(c)
}

func (b *Bridge) BuildRawInvokeTransaction(contractAddress string, entrypointSelector string, callData ...string) (rawTx interface{}, err error) {
	call := FunctionCall{
		ContractAddress:    HexToHash(contractAddress),
		EntryPointSelector: entrypointSelector,
		Calldata:           callData,
	}

	estimate, err := b.sendEstimateTx(call)
	if err != nil {
		return nil, err
	}
	invokeNonce, err := b.GetPoolNonce(b.defaultAccount.Address, "")
	if err != nil {
		return nil, err
	}

	return FunctionCallWithDetails{
		Call:   call,
		MaxFee: estimate,
		Nonce:  new(big.Int).SetUint64(invokeNonce),
	}, nil
}

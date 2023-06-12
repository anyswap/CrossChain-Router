package starknet

import (
	"errors"
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

const (
	Invoke   = "invoke"
	Estimate = "estimate"
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

	routerContract := b.GetRouterContract(multichainToken)
	functionCall := b.PrepFunctionCall(routerContract, args.Selector, swapInArgs.getCalldata())

	return b.BuildRawInvokeTx(functionCall, args)
}

func (b *Bridge) PrepFunctionCall(contractAddress string, entryPointSelector string, callData []string) FunctionCall {
	call := FunctionCall{
		ContractAddress:    HexToHash(contractAddress),
		EntryPointSelector: entryPointSelector,
		Calldata:           callData,
	}
	return call
}

func (b *Bridge) BuildRawInvokeTx(call FunctionCall, args *tokens.BuildTxArgs) (interface{}, error) {
	maxFee, err := b.GetMaxFee(call, args)
	if err != nil {
		return nil, err
	}

	nonce, err := b.GetPoolNonce(b.account.Address, "")
	if err != nil {
		return nil, err
	}

	return FunctionCallWithDetails{
		Call:   call,
		MaxFee: maxFee,
		Nonce:  new(big.Int).SetUint64(nonce),
	}, nil
}

func (b *Bridge) GetMaxFee(call FunctionCall, args *tokens.BuildTxArgs) (*big.Int, error) {
	c, err := b.BuildSignedInvokeTx(call, Estimate, args)
	if err != nil {
		return nil, err
	}
	return b.EstimateFee(c)
}

func (b *Bridge) BuildSignedInvokeTx(call FunctionCall, callType string, args *tokens.BuildTxArgs) (interface{}, error) {
	details, txHash, err := b.PrepExecDetails(call, callType, nil, args)
	if err != nil {
		return nil, err
	}
	rsv, err := b.SignInvokeTx(txHash, args)
	if err != nil {
		return nil, err
	}
	return b.PrepSignedInvokeTx(rsv, call, details)
}

func (b *Bridge) SignInvokeTx(txHash string, args *tokens.BuildTxArgs) (string, error) {
	mpcPubkey := router.GetMPCPublicKey(args.From)
	keyID, rsvs, err := b.MPCSign(mpcPubkey, txHash, "estimate fee txid"+args.SwapID)
	if err != nil {
		log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction failed", "keyID", keyID, "txid", args.SwapID, "err", err)
		return "", err
	}
	log.Info(b.ChainConfig.BlockChain+" MPCSignTransaction finished", "keyID", keyID, "estimate", args.SwapID)

	if len(rsvs) != 1 {
		return "", fmt.Errorf("get sign status require one rsv but have %v (keyID = %v)", len(rsvs), keyID)
	}
	return rsvs[0], nil
}

func (b *Bridge) PrepExecDetails(call FunctionCall, callType string, fee *big.Int, args *tokens.BuildTxArgs) (*ExecuteDetails, string, error) {
	var maxFee *big.Int
	switch callType {
	case Invoke:
		if fee != nil {
			maxFee = fee
		} else {
			estimate, err := b.GetMaxFee(call, args)
			if err != nil {
				return nil, "", err
			}
			maxFee = estimate
		}
	case Estimate:
		maxFee = MAXFEE
	default:
		return nil, "", errors.New("unsupported call type, should be one of estimate or invoke")
	}
	nonce, err := b.GetPoolNonce(b.account.Address, "")
	if err != nil {
		return nil, "", err
	}
	details := &ExecuteDetails{
		Nonce:  new(big.Int).SetUint64(nonce),
		MaxFee: maxFee,
	}

	txHashBN, err := b.computeTxHash(call, details)
	if err != nil {
		return nil, "", err
	}

	txHash := types.BigToHash(txHashBN).Hex()

	return details, txHash, nil
}

func (b *Bridge) PrepSignedInvokeTx(rsv string, call FunctionCall, details *ExecuteDetails) (interface{}, error) {
	r, s, v := DecodeSignature(common.FromHex(rsv))
	signature := ConvertSignature(r, s, v)
	calldata := fmtCalldataStrings([]FunctionCall{call})

	return rpcv02.BroadcastedInvokeV1Transaction{
		Version:       rpcv02.TransactionV1,
		Type:          TxTypeInvoke,
		MaxFee:        details.MaxFee,
		Nonce:         details.Nonce,
		Calldata:      calldata,
		Signature:     signature,
		SenderAddress: types.HexToHash(b.account.Address),
	}, nil
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

func (b *Bridge) computeTxHash(call FunctionCall, details *ExecuteDetails) (txHash *big.Int, err error) {
	txHash, err = b.TransactionHash(call, details.MaxFee, details.Nonce)
	if err != nil {
		return nil, err
	}
	return txHash, nil
}

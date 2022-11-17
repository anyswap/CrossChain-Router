package cosmos

import (
	"errors"
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
	retryRPCCount           = 3
	retryRPCInterval        = 1 * time.Second
	DefaultGasLimit  uint64 = 150000
	DefaultFee              = "500"
)

// BuildRawTransaction build raw tx
//
//nolint:gocyclo // ok
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

	routerMPC := b.GetRouterContract("")
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
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

	tokenCfg := b.GetTokenConfig(multichainToken)
	if tokenCfg == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	if receiver, amount, err := b.getReceiverAndAmount(args, multichainToken); err != nil {
		return nil, err
	} else {
		args.SwapValue = amount // SwapValue
		if extra, err := b.initExtra(args); err != nil {
			return nil, err
		} else {
			memo := fmt.Sprintf("Multichain_swapIn_%s_%d", args.SwapID, args.LogIndex)
			mpcPubkey := router.GetMPCPublicKey(args.From)
			if txBuilder, err := b.BuildTx(args.From, receiver, multichainToken, memo, mpcPubkey, amount, extra); err != nil {
				return nil, err
			} else {
				return &BuildRawTx{
					TxBuilder: &txBuilder,
					Extra:     extra,
				}, nil
			}
		}
	}
}

func (b *Bridge) initExtra(args *tokens.BuildTxArgs) (extra *tokens.AllExtras, err error) {
	denom := b.Denom
	extra = args.Extra
	if extra == nil {
		extra = &tokens.AllExtras{}
		args.Extra = extra
	}
	if extra.Sequence == nil {
		if extra.Sequence, err = b.GetSeq(args); err != nil {
			return nil, err
		}
	}
	if extra.AccountNum == nil {
		if accountNum, err := b.GetAccountNum(args); err != nil {
			return nil, err
		} else {
			extra.AccountNum = &accountNum
		}
	}
	if extra.Gas == nil {
		extra.Gas = &DefaultGasLimit
	}
	if extra.Fee == nil {
		fee := DefaultFee + denom
		extra.Fee = &fee
	}
	return extra, nil
}

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	if acc, err := b.GetBaseAccount(address); err != nil {
		return 0, err
	} else {
		if acc != nil {
			if sequence, err := strconv.ParseUint(acc.Account.Sequence, 10, 64); err == nil {
				return sequence, nil
			} else {
				return 0, err
			}
		}
		return 0, tokens.ErrRPCQueryError
	}
}

// GetSeq returns account tx sequence
func (b *Bridge) GetSeq(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

	if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) { // increase automatically
		nonce = b.GetSwapNonce(args.From)
		return &nonce, nil
	}

	for i := 0; i < retryRPCCount; i++ {
		nonce, err = b.GetPoolNonce(args.From, "pending")
		if err == nil {
			break
		}
		time.Sleep(retryRPCInterval)
	}
	if err != nil {
		return nil, err
	}
	nonce = b.AdjustNonce(args.From, nonce)
	return &nonce, nil
}

// GetSeq returns account tx sequence
func (b *Bridge) GetAccountNum(args *tokens.BuildTxArgs) (uint64, error) {
	if acc, err := b.GetBaseAccount(args.From); err != nil {
		return 0, err
	} else {
		if acc != nil {
			if accountNumber, err := strconv.ParseUint(acc.Account.AccountNumber, 10, 64); err == nil {
				return accountNumber, nil
			} else {
				return 0, err
			}
		}
		return 0, tokens.ErrRPCQueryError
	}
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount *big.Int, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
	if !b.IsValidAddress(receiver) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("swapout to invalid receiver")
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

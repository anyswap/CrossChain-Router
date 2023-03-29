package cosmos

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	retryRPCCount           = 3
	retryRPCInterval        = 1 * time.Second
	DefaultGasLimit  uint64 = 150000
	DefaultFee              = "500"

	cachedAccountNumberMap = make(map[string]uint64)

	numberPattern = regexp.MustCompile(`^\d+(?:.\d+)?$`)
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

	routerMPC, err := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
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
			memo := args.GetUniqueSwapIdentifier()
			mpcPubkey := router.GetMPCPublicKey(args.From)
			if txBuilder, err := b.BuildTx(args, receiver, multichainToken, memo, mpcPubkey, amount); err != nil {
				return nil, err
			} else {
				accountNumber, err := b.GetAccountNum(args.From)
				if err != nil {
					return nil, err
				}
				log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()),
					"identifier", args.Identifier, "swapID", args.SwapID,
					"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
					"from", args.From, "receiver", receiver,
					"accountNumber", accountNumber, "sequence", *extra.Sequence,
					"gasLimit", *extra.Gas, "replaceNum", args.GetReplaceNum(),
					"originValue", args.OriginValue, "swapValue", args.SwapValue,
					"gasFee", *extra.Fee, "bridgeFee", extra.BridgeFee,
				)
				return &BuildRawTx{
					TxBuilder:     txBuilder,
					AccountNumber: accountNumber,
					Sequence:      *extra.Sequence,
				}, nil
			}
		}
	}
}

func (b *Bridge) initExtra(args *tokens.BuildTxArgs) (extra *tokens.AllExtras, err error) {
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
	if extra.Gas == nil {
		extra.Gas = &DefaultGasLimit
	}
	if extra.Fee == nil {
		fee := b.getDefaultFee()
		replaceNum := args.GetReplaceNum()
		if replaceNum > 0 {
			serverCfg := params.GetRouterServerConfig()
			if serverCfg == nil {
				return nil, fmt.Errorf("no router server config")
			}
			coinsFee, err := ParseCoinsFee(fee)
			if err != nil {
				return nil, err
			}
			coinFee := coinsFee[0].Amount.BigInt()
			addPercent := serverCfg.PlusGasPricePercentage
			addPercent += replaceNum * serverCfg.ReplacePlusGasPricePercent
			if addPercent > serverCfg.MaxPlusGasPricePercentage {
				addPercent = serverCfg.MaxPlusGasPricePercentage
			}
			if addPercent > 0 {
				coinFee.Mul(coinFee, big.NewInt(int64(100+addPercent)))
				coinFee.Div(coinFee, big.NewInt(100))
			}
			coinsFee[0].Amount = sdk.NewIntFromBigInt(coinFee)
			adjustFee := coinsFee.String()
			extra.Fee = &adjustFee
		} else {
			extra.Fee = &fee
		}
	}
	return extra, nil
}

func (b *Bridge) getDefaultFee() string {
	fee := DefaultFee
	serverCfg := params.GetRouterServerConfig()
	if serverCfg != nil {
		if cfgFee, exist := serverCfg.DefaultFee[b.ChainConfig.ChainID]; exist {
			fee = cfgFee
		}
	}
	if is_numeric(fee) {
		fee += b.Denom
	}
	return fee
}

func is_numeric(word string) bool {
	return numberPattern.MatchString(word)
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

// GetAccountNum get account number
func (b *Bridge) GetAccountNum(account string) (uint64, error) {
	if accNo := cachedAccountNumberMap[account]; accNo > 0 {
		return accNo, nil
	}
	if acc, err := b.GetBaseAccount(account); err != nil {
		return 0, err
	} else {
		if acc != nil {
			if accountNumber, err := strconv.ParseUint(acc.Account.AccountNumber, 10, 64); err == nil {
				cachedAccountNumberMap[account] = accountNumber
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
	totalAmount := tokens.ConvertTokenValue(args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals)
	args.Extra.BridgeFee = new(big.Int).Sub(totalAmount, amount)
	return receiver, amount, err
}

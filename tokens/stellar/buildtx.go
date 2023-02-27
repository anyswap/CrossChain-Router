package stellar

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/stellar/go/txnbuild"
)

// BuildRawTransaction build raw tx
//
//nolint:funlen,gocyclo // ok
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
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

	assetI, exist := assetMap.Load(token.ContractAddress)
	if !exist {
		return nil, fmt.Errorf("non exist asset %v", token.ContractAddress)
	}
	asset := assetI.(txnbuild.Asset)

	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return nil, err
	}
	args.SwapValue = amount // SwapValue
	amt := getPaymentAmount(amount, token)

	fromAccount, err := b.GetAccount(args.From)
	if err != nil {
		return nil, err
	}

	// check XLM
	if !b.checkXlmBalanceEnough(fromAccount) {
		return nil, tokens.ErrMissTokenConfig
	}

	b.setExtraArgs(args)

	memo := buildMemo(args)
	return NewUnsignedPaymentTransaction(args, fromAccount, b.NetworkStr, receiver, amt, memo, asset)
}

func buildMemo(args *tokens.BuildTxArgs) *txnbuild.MemoHash {
	var memo txnbuild.MemoHash

	swapID := args.SwapID

	if common.IsHexHash(swapID) {
		memo = txnbuild.MemoHash(common.HexToHash(swapID))
	}

	return &memo
}

func getPaymentAmount(amount *big.Int, token *tokens.TokenConfig) string {
	value := float64(amount.Int64()) / math.Pow10(int(token.Decimals))
	return fmt.Sprintf("%.7f", value)
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount *big.Int, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
	if receiver == "" || !b.IsValidAddress(args.Bind) {
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

func (b *Bridge) setExtraArgs(args *tokens.BuildTxArgs) *tokens.AllExtras {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	extra := args.Extra
	extra.EthExtra = nil // clear this which may be set in replace job

	if extra.Fee == nil {
		feeRes := b.GetFee()
		fee := strconv.Itoa(feeRes)
		extra.Fee = &fee
	}

	if extra.Sequence == nil {
		maxTime := uint64(txnbuild.NewTimeout(txTimeout).MaxTime)
		extra.Sequence = &maxTime
	}

	return extra
}

// NewUnsignedPaymentTransaction build stellar payment tx
func NewUnsignedPaymentTransaction(args *tokens.BuildTxArgs,
	from txnbuild.Account, network,
	dest, amt string, memo txnbuild.Memo, asset txnbuild.Asset,
) (*txnbuild.Transaction, error) {
	baseFee, err := strconv.ParseInt(*args.Extra.Fee, 10, 64)
	if err != nil {
		return nil, err
	}
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        from,
			IncrementSequenceNum: true,
			BaseFee:              baseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, int64(*args.Extra.Sequence))},
			Memo:                 memo,
			Operations: []txnbuild.Operation{
				&txnbuild.Payment{
					Destination: dest,
					Amount:      amt,
					Asset:       asset,
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	hash, err := tx.Hash(network)
	if err != nil {
		return nil, err
	}
	log.Info("Build unsigned payment tx success",
		"destination", dest, "amount", amt, "memo", memo,
		"fee", baseFee, "Sequence", *args.Extra.Sequence,
		"signing hash", hash)

	return tx, nil
}

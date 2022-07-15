package ripple

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/crypto"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
)

var (
	defaultFee     int64 = 10
	accountReserve       = big.NewInt(10000000)
)

// BuildRawTransaction build raw tx
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
	asset := assetI.(*data.Asset)

	receiver, toTag, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return nil, err
	}
	args.SwapValue = amount // SwapValue

	amt, err := getPaymentAmount(amount, token)
	if err != nil {
		return nil, err
	}

	if asset.IsNative() {
		needAmount := new(big.Int).Add(amount, b.getMinReserveFee())
		err = b.checkNativeBalance(args.From, needAmount, true)
		if err != nil {
			return nil, err
		}
		err = b.checkNativeBalance(receiver, amount, false)
		if err != nil {
			return nil, err
		}
	} else {
		err = b.checkNativeBalance(receiver, nil, false)
		if err != nil {
			return nil, err
		}
		err = b.checkNonNativeBalance(asset.Currency, asset.Issuer, args.From, receiver, amt)
		if err != nil {
			return nil, err
		}
	}

	extra, err := b.setExtraArgs(args)
	if err != nil {
		return nil, err
	}

	ripplePubKey := ImportPublicKey(common.FromHex(mpcPubkey))
	memo := args.GetUniqueSwapIdentifier()
	return NewUnsignedPaymentTransaction(
		ripplePubKey, nil, uint32(*extra.Sequence),
		receiver, toTag, amt.String(), *extra.Fee, memo, "", 0)
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, destTag *uint32, amount *big.Int, err error) {
	receiver, destTag, err = GetAddressAndTag(args.Bind)
	if err != nil {
		return receiver, destTag, amount, err
	}
	if receiver == "" || !b.IsValidAddress(args.Bind) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, destTag, amount, errors.New("can not swapout to empty or invalid receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	if fromBridge == nil {
		return receiver, destTag, amount, tokens.ErrNoBridgeForChainID
	}
	erc20SwapInfo := args.ERC20SwapInfo
	fromTokenCfg := fromBridge.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", erc20SwapInfo.Token)
		return receiver, destTag, amount, tokens.ErrMissTokenConfig
	}
	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return receiver, destTag, amount, tokens.ErrMissTokenConfig
	}
	amount = tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	return receiver, destTag, amount, err
}

func getPaymentAmount(amount *big.Int, token *tokens.TokenConfig) (*data.Amount, error) {
	assetI, exist := assetMap.Load(token.ContractAddress)
	if !exist {
		return nil, fmt.Errorf("non exist asset %v", token.ContractAddress)
	}
	asset := assetI.(*data.Asset)

	currencyI, exist := currencyMap.Load(asset.Currency)
	if !exist {
		return nil, fmt.Errorf("non exist currency %v", asset.Currency)
	}
	currency := currencyI.(*data.Currency)

	if !amount.IsInt64() {
		return nil, fmt.Errorf("amount value %v is overflow of type int64", amount)
	}

	if currency.IsNative() { // native XRP
		return data.NewAmount(amount.Int64())
	}

	issuerI, exist := issuerMap.Load(asset.Issuer)
	if !exist {
		return nil, fmt.Errorf("non exist issuer %v", asset.Issuer)
	}
	issuer := issuerI.(*data.Account)

	// get a Value of amount*10^(-decimals)
	value, err := data.NewNonNativeValue(amount.Int64(), -int64(token.Decimals))
	if err != nil {
		log.Error("getPaymentAmount failed", "currency", asset.Currency, "issuer", asset.Issuer, "amount", amount, "decimals", token.Decimals, "err", err)
		return nil, err
	}

	return &data.Amount{
		Value:    value,
		Currency: *currency,
		Issuer:   *issuer,
	}, nil
}

func (b *Bridge) getMinReserveFee() *big.Int {
	config := params.GetRouterConfig()
	if config == nil {
		return big.NewInt(0)
	}
	minReserve := params.GetMinReserveFee(b.ChainConfig.ChainID)
	if minReserve == nil {
		minReserve = big.NewInt(100000) // default 0.1 XRP
	}
	return minReserve
}

func (b *Bridge) setExtraArgs(args *tokens.BuildTxArgs) (*tokens.AllExtras, error) {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	extra := args.Extra
	extra.EthExtra = nil // clear this which may be set in replace job

	if extra.Sequence == nil {
		seq, err := b.GetSeq(args)
		if err != nil {
			log.Warn("get sequence failed", "err", err)
			return nil, err
		}
		extra.Sequence = seq
	}

	if extra.Fee == nil {
		feeRes, err := b.GetFee()
		if err != nil {
			log.Warn("get fee failed", "err", err)
			return nil, err
		}
		feeAmount := feeRes.Drops.MinimumFee.Drops()
		if feeAmount < defaultFee {
			feeAmount = defaultFee
		}
		feeVal, _ := data.NewNativeValue(feeAmount)
		fee := feeVal.String()
		extra.Fee = &fee
	}

	return extra, nil
}

func (b *Bridge) checkNativeBalance(account string, amount *big.Int, isPay bool) error {
	balance, err := b.GetBalance(account)
	if err != nil && balance == nil {
		balance = big.NewInt(0)
	}

	remain := balance
	if amount != nil {
		if isPay {
			remain = new(big.Int).Sub(balance, amount)
		} else {
			remain = new(big.Int).Add(balance, amount)
		}
	}

	if remain.Cmp(accountReserve) < 0 {
		if isPay {
			return fmt.Errorf("insufficient native balance, sender: %v", account)
		}
		return fmt.Errorf("insufficient native balance, receiver: %v", account)
	}

	return nil
}

func (b *Bridge) checkNonNativeBalance(currency, issuer, account, receiver string, amount *data.Amount) error {
	if !params.IsSwapServer {
		return nil
	}
	_, err := b.GetAccountLine(currency, issuer, receiver)
	if err != nil {
		log.Error("get receiver account line failed", "currency", currency, "issuer", issuer, "receiver", receiver, "err", err)
		return fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, "get receiver account line failed")
	}

	if issuer == account {
		return nil
	}

	accl, err := b.GetAccountLine(currency, issuer, account)
	if err != nil {
		return fmt.Errorf("sender account line: %w", err)
	}
	if accl.Balance.Value.Compare(*amount.Value) < 0 {
		return fmt.Errorf("insufficient %v balance, issuer: %v, account: %v", currency, issuer, account)
	}

	return nil
}

// GetTxBlockInfo impl NonceSetter interface
func (b *Bridge) GetTxBlockInfo(txHash string) (blockHeight, blockTime uint64) {
	txStatus, err := b.GetTransactionStatus(txHash)
	if err != nil {
		return 0, 0
	}
	return txStatus.BlockHeight, txStatus.BlockTime
}

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	var nonce uint32
	account, err := b.GetAccount(address)
	if err != nil {
		return 0, fmt.Errorf("cannot get account, %w", err)
	}
	if seq := account.AccountData.Sequence; seq != nil {
		nonce = *seq
	}
	return uint64(nonce), nil
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

	nonce, err = b.GetPoolNonce(args.From, "pending")
	if err != nil {
		return nil, err
	}
	nonce = b.AdjustNonce(args.From, nonce)
	return &nonce, nil
}

// NewUnsignedPaymentTransaction build ripple payment tx
func NewUnsignedPaymentTransaction(
	key crypto.Key, keyseq *uint32, txseq uint32,
	dest string, destinationTag *uint32,
	amt, fee, memo, path string, flags uint32,
) (data.Transaction, error) {
	destination, err := data.NewAccountFromAddress(dest)
	if err != nil {
		return nil, err
	}
	amount, err := data.NewAmount(amt)
	if err != nil {
		return nil, err
	}
	tx := &data.Payment{
		Destination:    *destination,
		Amount:         *amount,
		DestinationTag: destinationTag,
	}
	tx.TransactionType = data.PAYMENT

	txFlags := data.TransactionFlag(flags)
	tx.Flags = &txFlags

	if memo != "" {
		memoStr := new(data.Memo)
		memoStr.Memo.MemoData = []byte(memo)
		tx.Memos = append(tx.Memos, *memoStr)
	}

	if path != "" {
		tx.Paths, err = ParsePaths(path)
		if err != nil {
			return nil, err
		}
	}

	base := tx.GetBase()

	base.Sequence = txseq

	fei, err := data.NewValue(fee, true)
	if err != nil {
		return nil, err
	}
	base.Fee = *fei

	copy(base.Account[:], key.Id(keyseq))

	tx.InitialiseForSigning()
	copy(tx.GetPublicKey().Bytes(), key.Public(keyseq))
	hash, msg, err := data.SigningHash(tx)
	if err != nil {
		return nil, err
	}
	log.Info("Build unsigned payment tx success",
		"destination", dest, "amount", amt, "memo", memo,
		"fee", fee, "sequence", txseq, "txflags", txFlags.String(),
		"signing hash", hash.String(), "blob", fmt.Sprintf("%X", msg))

	return tx, nil
}

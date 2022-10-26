package aptos

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	maxFee              string = "2000"
	defaultGasUnitPrice string = "100"
	timeout_seconds     int64  = 600
)

// BuildRawTransaction impl
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.SwapType != tokens.ERC20SwapType {
		return nil, tokens.ErrSwapTypeNotSupported
	}
	if args.ERC20SwapInfo == nil || args.ERC20SwapInfo.TokenID == "" {
		return nil, tokens.ErrEmptyTokenID
	}

	if args.From == "" {
		return nil, errors.New("forbid empty sender")
	}
	routerMPC, err := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}

	erc20SwapInfo := args.ERC20SwapInfo
	tokenID := erc20SwapInfo.TokenID
	chainID := b.ChainConfig.ChainID

	multichainToken := router.GetCachedMultichainToken(tokenID, chainID)
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", tokenID, "chainID", chainID)
		return nil, tokens.ErrMissTokenConfig
	}

	tokenCfg := b.GetTokenConfig(multichainToken)
	if tokenCfg == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	err = b.HasRegisterAptosCoin(args.Bind, tokenCfg)
	if err != nil {
		return nil, err
	}

	err = b.SetExtraArgs(args, tokenCfg)
	if err != nil {
		return nil, err
	}

	tx, err := b.BuildSwapinTransferTransaction(args, tokenCfg)
	if err != nil {
		return nil, err
	}

	if params.IsSwapServer {
		mpcPubkey := router.GetMPCPublicKey(args.From)
		if mpcPubkey == "" {
			return nil, tokens.ErrMissMPCPublicKey
		}
		// Simulated transactions must have a non-valid signature
		err = b.SimulateTranscation(tx, mpcPubkey)
		if err != nil {
			return nil, fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, err)
		}
	}
	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "bind", args.Bind, "nonce", tx.SequenceNumber,
		"gasPrice", tx.GasUnitPrice, "gasCurrency", tx.GasCurrencyCode,
		"originValue", args.OriginValue, "swapValue", args.SwapValue,
		"replaceNum", args.GetReplaceNum(), "tokenID", tokenID,
	}
	log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()), ctx...)
	return tx, nil
}

func (b *Bridge) HasRegisterAptosCoin(address string, tokenCfg *tokens.TokenConfig) error {
	result, err := b.GetAccountBalance(address, tokenCfg.ContractAddress)
	if err != nil {
		return err
	}
	if result == nil || result.Data == nil {
		return fmt.Errorf("%s not register coin %s ", address, tokenCfg.ContractAddress)
	}
	if !strings.EqualFold(tokenCfg.ContractAddress, tokenCfg.Extra) {
		result, err := b.GetAccountBalance(address, tokenCfg.Extra)
		if err != nil {
			return err
		}
		if result == nil || result.Data == nil {
			return fmt.Errorf("%s not register coin %s ", address, tokenCfg.Extra)
		}
	}
	return nil
}

func (b *Bridge) SetExtraArgs(args *tokens.BuildTxArgs, tokenCfg *tokens.TokenConfig) error {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	extra := args.Extra
	extra.EthExtra = nil // clear this which may be set in replace job
	if extra.Sequence == nil {
		sequence, err := b.getAccountNonce(args)
		if err != nil {
			if strings.Contains(err.Error(), "AptosError:") {
				return fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, err)
			}
			return err
		}
		extra.Sequence = sequence
	}
	if extra.BlockHash == nil {
		// 10 min
		expiration := strconv.FormatInt(time.Now().Unix()+timeout_seconds, 10)
		extra.BlockHash = &expiration
	}
	if extra.Gas == nil {
		gas, err := strconv.ParseUint(b.getGasPrice(), 10, 64)
		if err != nil {
			return err
		}
		extra.Gas = &gas
	}
	if extra.Fee == nil {
		maxGasFee := b.getMaxFee()
		extra.Fee = &maxGasFee
	}
	log.Info("Build tx with extra args", "extra", extra)
	return nil
}

// BuildSwapinTransferTransaction build swapin transfer tx
func (b *Bridge) BuildSwapinTransferTransaction(args *tokens.BuildTxArgs, tokenCfg *tokens.TokenConfig) (*Transaction, error) {
	receiver, amount, err := b.getReceiverAndAmount(args, tokenCfg.ContractAddress)
	if err != nil {
		return nil, err
	}
	tx := &Transaction{
		Sender:                  args.From,
		SequenceNumber:          strconv.FormatUint(*args.Extra.Sequence, 10),
		MaxGasAmount:            *args.Extra.Fee,
		GasUnitPrice:            strconv.FormatUint(*args.Extra.Gas, 10),
		ExpirationTimestampSecs: *args.Extra.BlockHash,
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(args.From, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SWAPIN),
			TypeArguments: []string{tokenCfg.Extra, tokenCfg.ContractAddress},
			Arguments:     []interface{}{receiver, strconv.FormatUint(amount, 10), args.SwapID, args.FromChainID.String()},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildSwapinTransferTransactionForScript(router, coin, poolcoin, receiver, amount, swapID, FromChainID string) (*Transaction, error) {
	account, err := b.GetAccount(router)
	if err != nil {
		return nil, err
	}
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  router,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            defaultGasUnitPrice,
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(router, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SWAPIN),
			TypeArguments: []string{coin, poolcoin},
			Arguments:     []interface{}{receiver, amount, swapID, FromChainID},
		},
	}
	return tx, nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount uint64, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
	if !b.IsValidAddress(receiver) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("can not swapout to invalid receiver")
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
	swapValue := tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	if !swapValue.IsUint64() {
		return receiver, amount, tokens.ErrTxWithWrongValue
	}
	return receiver, swapValue.Uint64(), err
}

func (b *Bridge) getGasPrice() string {
	estimateGasPrice, err := b.EstimateGasPrice()
	if err == nil {
		log.Debugln("estimateGasPrice", "GasPrice", estimateGasPrice.GasPrice)
		configGasPrice := params.GetMaxGasPrice(b.ChainConfig.ChainID)
		if configGasPrice == nil {
			return strconv.Itoa(estimateGasPrice.GasPrice)
		} else {
			return strconv.FormatUint(configGasPrice.Uint64(), 10)
		}
	} else {
		log.Debugln("estimateGasPrice", "GasPrice", defaultGasUnitPrice)
		return defaultGasUnitPrice
	}
}

func (b *Bridge) getMaxFee() string {
	maxGasFee := params.GetMaxGasLimit(b.ChainConfig.ChainID)
	if maxGasFee == 0 {
		return maxFee
	} else {
		return strconv.FormatUint(maxGasFee, 10)
	}
}

func (b *Bridge) getAccountNonce(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

	res, err := b.GetPoolNonce(args.From, "")
	if err != nil {
		return nil, err
	}

	nonce = b.AdjustNonce(args.From, res)
	return &nonce, nil
}

func (b *Bridge) BuildDeployModuleTransaction(address, packagemetadata string, moduleHexs []string) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// fee, _ := strconv.Atoi(maxFee)

	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:         address,
		SequenceNumber: account.SequenceNumber,
		// MaxGasAmount:            strconv.Itoa(fee * len(moduleHexs)),
		MaxGasAmount:            "20000",
		GasUnitPrice:            "1000",
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      PUBLISH_PACKAGE,
			TypeArguments: []string{},
			Arguments:     []interface{}{packagemetadata, moduleHexs},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildRegisterPoolCoinTransaction(address, underlyingCoin, poolCoin, poolCoinName, poolCoinSymbol string, decimals uint8) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, CONTRACT_NAME_POOL, CONTRACT_FUNC_REGISTER_COIN),
			TypeArguments: []string{underlyingCoin, poolCoin},
			Arguments:     []interface{}{common.ToHex([]byte(poolCoinName)), common.ToHex([]byte(poolCoinSymbol)), decimals},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildSetCoinTransaction(address, coin string, coinType uint8) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SET_COIN),
			TypeArguments: []string{coin},
			Arguments:     []interface{}{coinType},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildSetStatusTransaction(address string, status uint8) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SET_COIN),
			TypeArguments: []string{},
			Arguments:     []interface{}{status},
		},
	}
	return tx, nil
}
func (b *Bridge) BuildSetPoolcoinCapTransaction(address, coin string) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SET_POOLCOIN_CAP),
			TypeArguments: []string{coin},
			Arguments:     []interface{}{},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildManagedCoinInitializeTransaction(address, coin, poolCoinName, poolCoinSymbol string, decimals uint8, monitor_supply bool) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      "0x1::managed_coin::initialize",
			TypeArguments: []string{coin},
			Arguments:     []interface{}{common.ToHex([]byte(poolCoinName)), common.ToHex([]byte(poolCoinSymbol)), decimals, false},
		},
	}
	// common.ToHex([]byte{byte(decimals)})
	return tx, nil
}

func (b *Bridge) BuildRegisterCoinTransaction(address, coin string) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}

	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      "0x1::managed_coin::register",
			TypeArguments: []string{coin},
			Arguments:     []interface{}{},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildMintCoinTransaction(minter, toaddress, coin string, amount uint64) (*Transaction, error) {
	account, err := b.GetAccount(minter)
	if err != nil {
		return nil, err
	}

	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  minter,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      "0x1::managed_coin::mint",
			TypeArguments: []string{coin},
			Arguments:     []interface{}{toaddress, strconv.FormatUint(amount, 10)},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildSwapoutTransaction(sender, router, coin, toAddress, tochainId string, amount uint64) (*Transaction, error) {
	account, err := b.GetAccount(sender)
	if err != nil {
		return nil, err
	}

	// 10 min
	timeout := time.Now().Unix() + 600
	tx := &Transaction{
		Sender:                  sender,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(router, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SWAPOUT),
			TypeArguments: []string{coin},
			Arguments:     []interface{}{strconv.FormatUint(amount, 10), toAddress, tochainId},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildTestUnderlyingCoinMintTransaction(minter, toaddress, coin string, amount uint64) (*Transaction, error) {
	account, err := b.GetAccount(minter)
	if err != nil {
		return nil, err
	}

	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  minter,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(minter, strings.Split(coin, "::")[1], "mint"),
			TypeArguments: []string{},
			Arguments:     []interface{}{toaddress, strconv.FormatUint(amount, 10)},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildDepositTransaction(sender, pool, underlying, anycoin string, amount uint64) (*Transaction, error) {
	account, err := b.GetAccount(sender)
	if err != nil {
		return nil, err
	}

	// 10 min
	timeout := time.Now().Unix() + 600
	tx := &Transaction{
		Sender:                  sender,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(pool, CONTRACT_NAME_POOL, CONTRACT_FUNC_DEPOSIT),
			TypeArguments: []string{underlying, anycoin},
			Arguments:     []interface{}{strconv.FormatUint(amount, 10)},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildWithdrawTransaction(sender, pool, underlying, anycoin string, amount uint64) (*Transaction, error) {
	account, err := b.GetAccount(sender)
	if err != nil {
		return nil, err
	}

	// 10 min
	timeout := time.Now().Unix() + 600
	tx := &Transaction{
		Sender:                  sender,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(pool, CONTRACT_NAME_POOL, CONTRACT_FUNC_WITHDRAW),
			TypeArguments: []string{anycoin, underlying},
			Arguments:     []interface{}{strconv.FormatUint(amount, 10)},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildCopyCapTransaction(address, coin string) (*Transaction, error) {
	account, err := b.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            b.getGasPrice(),
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, coin, CONTRACT_FUNC_SET_UNDERLYING_CAP),
			TypeArguments: []string{},
			Arguments:     []interface{}{},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildTransferTransaction(sender, coin, receiver, amount string) (*Transaction, error) {
	account, err := b.GetAccount(sender)
	if err != nil {
		return nil, err
	}
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  sender,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            defaultGasUnitPrice,
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      "0x1::aptos_account::transfer",
			TypeArguments: []string{},
			Arguments:     []interface{}{receiver, amount},
		},
	}
	return tx, nil
}

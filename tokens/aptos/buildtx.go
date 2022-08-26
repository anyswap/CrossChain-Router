package aptos

import (
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	maxFee              string = "2000"
	defaultGasUnitPrice string = "1"
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

	tx, err := b.BuildSwapinTransferTransaction(args, tokenCfg)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount uint64, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
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

// BuildSwapinTransferTransaction build swapin transfer tx
func (b *Bridge) BuildSwapinTransferTransaction(args *tokens.BuildTxArgs, tokenCfg *tokens.TokenConfig) (*Transaction, error) {
	receiver, amount, err := b.getReceiverAndAmount(args, tokenCfg.ContractAddress)
	if err != nil {
		return nil, err
	}
	routerInfo, err := router.GetTokenRouterInfo(tokenCfg.TokenID, b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	account, err := b.Client.GetAccount(routerInfo.RouterMPC)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  routerInfo.RouterMPC,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            defaultGasUnitPrice,
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(routerInfo.RouterMPC, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SWAPIN),
			TypeArguments: []string{tokenCfg.ContractAddress, tokenCfg.Extra},
			Arguments:     []string{receiver, strconv.FormatUint(amount, 10), args.SwapID, args.FromChainID.String()},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildDeployModuleTransaction(address, moduleHex string) (*ModuleTransaction, error) {
	account, err := b.Client.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &ModuleTransaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            defaultGasUnitPrice,
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &ModulePayload{
			Type: MODULE_PAYLOAD,
			Modules: &[]ModuleDefine{
				{
					Bytecode: moduleHex,
				},
			},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildRegisterPoolCoinTransaction(address, underlyingCoin, poolCoin, poolCoinName, poolCoinSymbol string, decimals int) (*Transaction, error) {
	account, err := b.Client.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            defaultGasUnitPrice,
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, CONTRACT_NAME_POOL, CONTRACT_FUNC_REGISTER_COIN),
			TypeArguments: []string{underlyingCoin, poolCoin},
			Arguments:     []string{poolCoinName, poolCoinSymbol, strconv.Itoa(decimals)},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildSetCoinTransaction(address, coin string, coinType int) (*Transaction, error) {
	account, err := b.Client.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            defaultGasUnitPrice,
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SET_COIN),
			TypeArguments: []string{coin},
			Arguments:     []string{strconv.Itoa(coinType)},
		},
	}
	return tx, nil
}

func (b *Bridge) BuildSetPoolcoinCapTransaction(address, coin string) (*Transaction, error) {
	account, err := b.Client.GetAccount(address)
	if err != nil {
		return nil, err
	}
	// 10 min
	timeout := time.Now().Unix() + timeout_seconds
	tx := &Transaction{
		Sender:                  address,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            maxFee,
		GasUnitPrice:            defaultGasUnitPrice,
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      GetRouterFunctionId(address, CONTRACT_NAME_ROUTER, CONTRACT_FUNC_SET_POOLCOIN_CAP),
			TypeArguments: []string{coin},
			Arguments:     []string{},
		},
	}
	return tx, nil
}

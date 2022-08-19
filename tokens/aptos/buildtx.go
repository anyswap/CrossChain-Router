package aptos

import (
	"strconv"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
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
	_, err = b.Client.SubmitTranscation(tx)
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
	timeout := time.Now().Unix() + 600
	tx := &Transaction{
		Sender:                  routerInfo.RouterMPC,
		SequenceNumber:          account.SequenceNumber,
		MaxGasAmount:            "1000",
		GasUnitPrice:            "1",
		ExpirationTimestampSecs: strconv.FormatInt(timeout, 10),
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      "swapin",
			TypeArguments: []string{tokenCfg.ContractAddress},
			Arguments:     []string{receiver, strconv.FormatUint(amount, 10), args.SwapID, args.FromChainID.String()},
		},
	}
	return tx, nil
}

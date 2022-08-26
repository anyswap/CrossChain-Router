package aptos

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if len(msgHashes) < 1 {
		return fmt.Errorf("must provide msg hash")
	}
	return nil
}

// VerifyTransaction impl
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifySwapoutTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

//nolint:gocyclo,funlen // ok
func (b *Bridge) verifySwapoutTx(txHash string, _ int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = 0                             // LogIndex always 0 (do not support multiple in one tx)
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	tx, err := b.GetTransaction(txHash)
	if err != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return swapInfo, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*TransactionInfo)
	if !ok {
		return swapInfo, errTxResultType
	}

	if !txres.Success {
		return swapInfo, tokens.ErrTxWithWrongStatus
	}

	if !allowUnstable {
		h, errf := b.GetLatestBlockNumber()
		if errf != nil {
			return swapInfo, errf
		}
		ledger, _ := strconv.ParseUint(txres.Version, 10, 64)

		if h < ledger+b.GetChainConfig().Confirmations {
			return swapInfo, tokens.ErrTxNotStable
		}
		if h < b.ChainConfig.InitialHeight {
			return swapInfo, tokens.ErrTxBeforeInitialHeight
		}
	}
	err = b.verifySwapoutEvents(swapInfo, txres)
	if err != nil {
		return swapInfo, err
	}

	err = b.checkSwapoutInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify swapout pass",
			"from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}
	return swapInfo, nil
}

func (b *Bridge) verifySwapoutEvents(swapInfo *tokens.SwapTxInfo, txInfo *TransactionInfo) error {
	routerProgramID := b.ChainConfig.RouterContract

	swapInfo.TxTo = strings.Split(txInfo.PayLoad.Function, SPLIT_SYMBOL)[0]

	if !params.AllowCallByContract() &&
		!common.IsEqualIgnoreCase(swapInfo.TxTo, b.ChainConfig.RouterContract) &&
		!params.IsInCallByContractWhitelist(b.ChainConfig.ChainID, swapInfo.TxTo) {
		return tokens.ErrTxWithWrongContract
	}
	var swapOutInfo *Event
	for _, event := range txInfo.Events {
		if common.IsEqualIgnoreCase(event.Type, GetRouterFunctionId(routerProgramID, CONTRACT_NAME_ROUTER, "SwapOutEvent")) {
			swapOutInfo = &event
		}
	}
	if swapOutInfo == nil {
		return tokens.ErrSwapoutLogNotFound
	}

	swapInfo.To = routerProgramID
	erc20SwapInfo := &tokens.ERC20SwapInfo{}
	swapInfo.ERC20SwapInfo = erc20SwapInfo
	// SwapOutEvent in Router.move
	// struct SwapOutEvent has drop, store {
	//     token: string::String,
	//     from: address,
	//     to: string::String,
	//     amount: u64,
	//     to_chain_id: u64
	// }
	swapInfo.Bind = swapOutInfo.Data["to"]
	swapInfo.From = swapOutInfo.Data["from"]
	erc20SwapInfo.Token = swapOutInfo.Data["token"]
	tokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	erc20SwapInfo.TokenID = tokenCfg.TokenID
	value, err := common.GetUint64FromStr(swapOutInfo.Data["amount"])
	if err != nil {
		return err
	}
	swapInfo.Value = new(big.Int).SetUint64(value)
	swapInfo.FromChainID = b.ChainConfig.GetChainID()
	toChainID, err := common.GetUint64FromStr(swapOutInfo.Data["to_chain_id"])
	if err != nil {
		return err
	}
	swapInfo.ToChainID = new(big.Int).SetUint64(toChainID)
	return nil
}

func (b *Bridge) checkSwapoutInfo(swapInfo *tokens.SwapTxInfo) error {
	if strings.EqualFold(swapInfo.From, swapInfo.To) {
		return tokens.ErrTxWithWrongSender
	}

	erc20SwapInfo := swapInfo.ERC20SwapInfo

	fromTokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil || erc20SwapInfo.TokenID == "" {
		return tokens.ErrMissTokenConfig
	}

	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, swapInfo.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", swapInfo.ToChainID, "txid", swapInfo.Hash)
		return tokens.ErrMissTokenConfig
	}

	toBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if toBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	toTokenCfg := toBridge.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		log.Warn("get token config failed", "chainID", swapInfo.ToChainID, "token", multichainToken)
		return tokens.ErrMissTokenConfig
	}

	if !tokens.CheckTokenSwapValue(swapInfo, fromTokenCfg.Decimals, toTokenCfg.Decimals) {
		return tokens.ErrTxWithWrongValue
	}

	bindAddr := swapInfo.Bind
	if !toBridge.IsValidAddress(bindAddr) {
		log.Warn("wrong bind address in swapin", "bind", bindAddr)
		return tokens.ErrWrongBindAddress
	}
	return nil
}

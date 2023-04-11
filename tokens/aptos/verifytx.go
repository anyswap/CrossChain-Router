package aptos

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	tx, ok := rawTx.(*Transaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	signingMessage, err := b.GetSigningMessage(tx)
	if err != nil {
		return fmt.Errorf("unable to encode message for signing: %w", err)
	}
	if !strings.EqualFold(*signingMessage, msgHashes[0]) {
		log.Trace("message hash mismatch", "want", *signingMessage, "have", msgHashes[0])
		return tokens.ErrMsgHashMismatch
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

// GetTransactionInfo get tx info (verify tx status and check stable)
func (b *Bridge) GetTransactionInfo(swapInfo *tokens.SwapTxInfo, txHash string, allowUnstable bool) (*TransactionInfo, error) {
	tx, err := b.GetTransaction(txHash)
	if err != nil {
		log.Debug(b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return nil, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*TransactionInfo)
	if !ok {
		return nil, errTxResultType
	}

	if !txres.Success {
		return nil, tokens.ErrTxWithWrongStatus
	}

	if !allowUnstable {
		h, errf := b.GetLatestBlockNumber()
		if errf != nil {
			return nil, errf
		}

		txHeight, errf := b.GetBlockNumberByVersion(txres.Version)
		if errf != nil {
			return nil, errf
		}
		swapInfo.Height = txHeight

		if h < txHeight+b.GetChainConfig().Confirmations {
			return nil, tokens.ErrTxNotStable
		}

		if txHeight < b.ChainConfig.InitialHeight {
			return nil, tokens.ErrTxBeforeInitialHeight
		}
	}

	return txres, nil
}

//nolint:gocyclo,funlen // ok
func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = txHash                            // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	txres, err := b.GetTransactionInfo(swapInfo, txHash, allowUnstable)
	if err != nil {
		return swapInfo, err
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
			"fromChainId", swapInfo.FromChainID.String(), "to", swapInfo.ToChainID.String(),
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}
	return swapInfo, nil
}

func (b *Bridge) verifySwapoutEvents(swapInfo *tokens.SwapTxInfo, txInfo *TransactionInfo) error {
	if swapInfo.LogIndex >= len(txInfo.Events) {
		return tokens.ErrLogIndexOutOfRange
	}
	swapOutInfo := &txInfo.Events[swapInfo.LogIndex]

	routerProgramID := b.ChainConfig.RouterContract

	if !common.IsEqualIgnoreCase(swapOutInfo.Type, GetRouterFunctionId(routerProgramID, CONTRACT_NAME_ROUTER, "SwapOutEvent")) {
		return tokens.ErrSwapoutLogNotFound
	}

	swapInfo.TxTo = strings.Split(txInfo.PayLoad.Function, SPLIT_SYMBOL)[0]

	if !params.AllowCallByContract() &&
		!common.IsEqualIgnoreCase(swapInfo.TxTo, b.ChainConfig.RouterContract) &&
		!params.IsInCallByContractWhitelist(b.ChainConfig.ChainID, swapInfo.TxTo) {
		return tokens.ErrTxWithWrongContract
	}

	swapInfo.To = routerProgramID
	erc20SwapInfo := &tokens.ERC20SwapInfo{}
	swapInfo.SwapInfo = tokens.SwapInfo{ERC20SwapInfo: erc20SwapInfo}
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

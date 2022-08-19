package aptos

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
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

	asset := "txres.TransactionWithMetaData.MetaData.DeliveredAmount.Asset().String()"
	token := b.GetTokenConfig(asset)
	if token == nil {
		return swapInfo, tokens.ErrMissTokenConfig
	}

	txRecipient := txres.PayLoad.Function
	depositAddress := b.GetRouterContract(asset)
	if !common.IsEqualIgnoreCase(txRecipient, depositAddress) {
		return swapInfo, tokens.ErrTxWithWrongReceiver
	}

	erc20SwapInfo := &tokens.ERC20SwapInfo{}
	erc20SwapInfo.Token = asset
	erc20SwapInfo.TokenID = token.TokenID
	swapInfo.SwapInfo = tokens.SwapInfo{ERC20SwapInfo: erc20SwapInfo}

	err = b.checkToken(token)
	if err != nil {
		return swapInfo, err
	}

	amt := tokens.ToBits("", token.Decimals)

	swapInfo.To = depositAddress // To
	swapInfo.From = txres.Sender // From
	swapInfo.Value = amt

	err = b.checkSwapoutInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify swapin pass",
			"asset", asset, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}
	return swapInfo, nil
}

func (b *Bridge) checkToken(token *tokens.TokenConfig) error {

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

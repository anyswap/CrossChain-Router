package stellar

import (
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/protocols/horizon/operations"
)

// RegisterSwap api
func (b *Bridge) RegisterSwap(txHash string, args *tokens.RegisterArgs) ([]*tokens.SwapTxInfo, []error) {
	swapType := args.SwapType
	logIndex := args.LogIndex

	switch swapType {
	case tokens.ERC20SwapType:
		return b.registerERC20SwapTx(txHash, logIndex)
	default:
		return nil, []error{tokens.ErrSwapTypeNotSupported}
	}
}

func (b *Bridge) registerERC20SwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	tx, err := b.GetTransaction(txHash)
	if err != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return nil, []error{tokens.ErrTxNotFound}
	}

	txres, ok := tx.(*hProtocol.Transaction)
	if !ok {
		return nil, []error{errTxResultType}
	}

	// Check tx status
	if !txres.Successful {
		return nil, []error{tokens.ErrTxWithWrongStatus}
	}

	opts, err := b.GetOperations(txHash)
	if err != nil || len(opts) != int(txres.OperationCount) {
		return nil, []error{tokens.ErrLogIndexOutOfRange}
	}
	startIndex, endIndex := 0, len(opts)
	if logIndex != 0 {
		if logIndex >= endIndex || logIndex < 0 {
			return nil, []error{tokens.ErrLogIndexOutOfRange}
		}
		startIndex = logIndex
		endIndex = logIndex + 1
	}
	errs := make([]error, 0)
	swapInfos := make([]*tokens.SwapTxInfo, 0)
	for i := startIndex; i < endIndex; i++ {
		op := getPaymentOperation(opts[i])
		if op == nil {
			continue
		}
		si, err := b.buildSwapInfoFromOperation(txres, op, i)
		errs = append(errs, err)
		swapInfos = append(swapInfos, si)
	}
	return swapInfos, errs
}

func getPaymentOperation(opt interface{}) *operations.Payment {
	op, ok := opt.(operations.Payment)
	if ok && op.GetType() == "payment" && op.TransactionSuccessful {
		return &op
	}
	return nil
}

func (b *Bridge) buildSwapInfoFromOperation(txres *hProtocol.Transaction, op *operations.Payment, logIndex int) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.ERC20SwapType
	swapInfo.Hash = txres.Hash
	swapInfo.LogIndex = logIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID()
	swapInfo.Height = uint64(txres.Ledger)

	assetKey := convertTokenID(op)
	token := b.GetTokenConfig(assetKey)
	if token == nil {
		return swapInfo, tokens.ErrMissTokenConfig
	}
	txRecipient := op.To
	depositAddress := b.GetRouterContract(assetKey)
	if !common.IsEqualIgnoreCase(txRecipient, depositAddress) {
		return swapInfo, tokens.ErrTxWithWrongReceiver
	}
	erc20SwapInfo := &tokens.ERC20SwapInfo{}
	erc20SwapInfo.Token = assetKey
	erc20SwapInfo.TokenID = token.TokenID
	swapInfo.SwapInfo = tokens.SwapInfo{ERC20SwapInfo: erc20SwapInfo}

	if success := checkSwapMemos(swapInfo, txres.Memo); !success {
		log.Warn("wrong memos", "memos", txres.Memo)
		return swapInfo, tokens.ErrWrongBindAddress
	}

	amt := tokens.ToBits(op.Amount, token.Decimals)
	if amt.Cmp(big.NewInt(0)) <= 0 {
		return swapInfo, tokens.ErrTxWithWrongValue
	}

	swapInfo.To = depositAddress
	swapInfo.From = op.From
	swapInfo.Value = amt

	err := b.checkSwapoutInfo(swapInfo)
	if err != nil {
		return nil, err
	}

	return swapInfo, nil
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

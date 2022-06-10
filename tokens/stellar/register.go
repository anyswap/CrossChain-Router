package stellar

import (
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
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
		op, ok := opts[i].(operations.Payment)
		if !ok || op.GetType() != "payment" || !op.TransactionSuccessful {
			continue
		}
		si, err := b.buildSwapInfoFromOperation(txres, &op, i)
		errs = append(errs, err)
		swapInfos = append(swapInfos, si)
	}
	return swapInfos, errs
}

func (b *Bridge) buildSwapInfoFromOperation(txres *hProtocol.Transaction, op *operations.Payment, logIndex int) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.ERC20SwapType
	swapInfo.Hash = txres.Hash
	swapInfo.LogIndex = logIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID()

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

	if success := parseSwapMemos(swapInfo, txres.Memo); !success {
		log.Debug("wrong memos", "memos", txres.Memo)
		return swapInfo, tokens.ErrWrongBindAddress
	}

	amt := tokens.ToBits(op.Amount, token.Decimals)
	if amt.Cmp(big.NewInt(0)) <= 0 {
		return swapInfo, tokens.ErrTxWithWrongValue
	}

	swapInfo.To = depositAddress
	swapInfo.From = op.From
	swapInfo.Value = amt
	return swapInfo, nil
}

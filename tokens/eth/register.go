package eth

import (
	"github.com/anyswap/CrossChain-Router/log"
	"github.com/anyswap/CrossChain-Router/tokens"
	"github.com/anyswap/CrossChain-Router/types"
)

// RegisterSwap api
func (b *Bridge) RegisterSwap(txHash string, args *tokens.RegisterArgs) ([]*tokens.SwapTxInfo, []error) {
	swapType := args.SwapType
	logIndex := args.LogIndex

	switch swapType {
	case tokens.RouterSwapType:
		return b.RegisterRouterSwapTx(txHash, logIndex)
	default:
		return nil, []error{tokens.ErrSwapTypeNotSupported}
	}
}

// RegisterRouterSwapTx impl
func (b *Bridge) RegisterRouterSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	commonInfo := &tokens.SwapTxInfo{RouterSwapInfo: &tokens.RouterSwapInfo{}}
	commonInfo.SwapType = tokens.RouterSwapType // SwapType
	commonInfo.Hash = txHash                    // Hash
	commonInfo.LogIndex = logIndex              // LogIndex

	txStatus := b.GetTransactionStatus(txHash)
	if txStatus.BlockHeight == 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrTxNotFound}
	}

	commonInfo.Height = txStatus.BlockHeight  // Height
	commonInfo.Timestamp = txStatus.BlockTime // Timestamp

	receipt, _ := txStatus.Receipt.(*types.RPCTxReceipt)
	err := b.verifyRouterSwapTxReceipt(commonInfo, receipt)
	if err != nil {
		return []*tokens.SwapTxInfo{commonInfo}, []error{err}
	}

	swapInfos := make([]*tokens.SwapTxInfo, 0)
	errs := make([]error, 0)
	startIndex, endIndex := 1, len(receipt.Logs)

	if logIndex != 0 {
		if logIndex >= endIndex || logIndex < 0 {
			return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrLogIndexOutOfRange}
		}
		startIndex = logIndex
		endIndex = logIndex + 1
	}

	for i := startIndex; i < endIndex; i++ {
		swapInfo := &tokens.SwapTxInfo{}
		*swapInfo = *commonInfo
		swapInfo.RouterSwapInfo = &tokens.RouterSwapInfo{}
		swapInfo.LogIndex = i // LogIndex
		err := b.verifyRouterSwapTxLog(swapInfo, receipt.Logs[i])
		switch err {
		case tokens.ErrSwapoutLogNotFound:
			continue
		case nil:
			err = b.checkRouterSwapInfo(swapInfo)
		default:
			log.Debug(b.ChainConfig.BlockChain+" register swap error", "txHash", txHash, "logIndex", swapInfo.LogIndex, "err", err)
		}
		swapInfos = append(swapInfos, swapInfo)
		errs = append(errs, err)
	}

	if len(swapInfos) == 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrSwapoutLogNotFound}
	}

	return swapInfos, errs
}

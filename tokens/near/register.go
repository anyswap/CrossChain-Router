package near

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
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
	log.Info("registerERC20SwapTx", "txhash:", txHash, "logIndex:", logIndex)
	commonInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	commonInfo.SwapType = tokens.ERC20SwapType          // SwapType
	commonInfo.Hash = txHash                            // Hash
	commonInfo.LogIndex = logIndex                      // LogIndex
	commonInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	receipts, err := b.getSwapTxReceipt(commonInfo, true)
	if err != nil {
		return []*tokens.SwapTxInfo{commonInfo}, []error{err}
	}
	log.Info("getSwapTxReceipt", "receipt:", receipts)

	swapInfos := make([]*tokens.SwapTxInfo, 0)
	errs := make([]error, 0)
	startIndex, endIndex := 1, len(receipts)

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
		swapInfo.ERC20SwapInfo = &tokens.ERC20SwapInfo{}
		swapInfo.LogIndex = i // LogIndex
		events, errv := b.fliterReceipts(&receipts[i])
		if errv != nil {
			continue
		}
		err = b.parseNep141SwapoutTxEvent(swapInfo, events)
		switch {
		case errors.Is(err, tokens.ErrSwapoutLogNotFound),
			errors.Is(err, tokens.ErrTxWithWrongTopics),
			errors.Is(err, tokens.ErrTxWithWrongContract):
		case err == nil:
			err = b.checkSwapoutInfo(swapInfo)
		default:
			log.Debug(b.ChainConfig.BlockChain+" register router swap error", "txHash", txHash, "logIndex", swapInfo.LogIndex, "err", err)
		}
		swapInfos = append(swapInfos, swapInfo)
		errs = append(errs, err)
	}

	if len(swapInfos) == 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrSwapoutLogNotFound}
	}
	return swapInfos, errs
}

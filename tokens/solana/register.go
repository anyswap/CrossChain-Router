package solana

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// RegisterSwap impl
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
	commonInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	commonInfo.SwapType = tokens.ERC20SwapType // SwapType
	commonInfo.Hash = txHash                   // Hash
	commonInfo.LogIndex = logIndex             // LogIndex
	isSpecifiedLogIndex := logIndex > 0

	txm, err := b.getTransactionMeta(commonInfo, true)
	if err != nil {
		return []*tokens.SwapTxInfo{commonInfo}, []error{err}
	}
	logMessages := txm.LogMessages

	if logIndex >= len(logMessages) || logIndex < 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrLogIndexOutOfRange}
	}

	routerProgramID := b.ChainConfig.RouterContract
	invokeStart := fmt.Sprintf("Program %s invoke [", routerProgramID)

	swapInfos := make([]*tokens.SwapTxInfo, 0)
	errs := make([]error, 0)

	for i, msg := range logMessages[logIndex:] {
		if !strings.HasPrefix(msg, invokeStart) {
			continue
		}
		swapInfo := &tokens.SwapTxInfo{}
		*swapInfo = *commonInfo
		swapInfo.ERC20SwapInfo = &tokens.ERC20SwapInfo{}
		swapInfo.LogIndex = i // LogIndex
		err = b.verifySwapoutLogs(swapInfo, logMessages)
		if err != nil {
			log.Debug(b.ChainConfig.BlockChain+" register router swap error", "txHash", txHash, "logIndex", swapInfo.LogIndex, "err", err)
		}
		swapInfos = append(swapInfos, swapInfo)
		errs = append(errs, err)
		if isSpecifiedLogIndex {
			break
		}
	}

	if len(swapInfos) == 0 {
		return []*tokens.SwapTxInfo{commonInfo}, []error{tokens.ErrSwapoutLogNotFound}
	}

	return swapInfos, errs
}

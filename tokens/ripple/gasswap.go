package ripple

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/websockets"
)

func (b *Bridge) registerGasSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	swapInfo, err := b.verifyGasSwapTx(txHash, logIndex, true)
	return []*tokens.SwapTxInfo{swapInfo}, []error{err}
}

func (b *Bridge) verifyGasSwapTx(txHash string, _ int, allowUnstable bool) (*tokens.SwapTxInfo, error) {

	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{GasSwapInfo: &tokens.GasSwapInfo{}}}
	swapInfo.SwapType = tokens.GasSwapType  // SwapType
	swapInfo.Hash = strings.ToLower(txHash) // Hash
	swapInfo.LogIndex = 0                   // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID()

	tx, err := b.GetTransaction(txHash)
	if err != nil {
		log.Debug("[verifyGasSwapTx] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return swapInfo, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*websockets.TxResult)
	if !ok {
		return swapInfo, errTxResultType
	}

	if !txres.Validated {
		return swapInfo, tokens.ErrTxIsNotValidated
	}

	if !allowUnstable {
		h, errf := b.GetLatestBlockNumber()
		if errf != nil {
			return swapInfo, errf
		}

		if h < uint64(txres.TransactionWithMetaData.LedgerSequence)+b.GetChainConfig().Confirmations {
			return swapInfo, tokens.ErrTxNotStable
		}
		if h < b.ChainConfig.InitialHeight {
			return swapInfo, tokens.ErrTxBeforeInitialHeight
		}
	}

	// Check tx status
	if !txres.TransactionWithMetaData.MetaData.TransactionResult.Success() {
		return swapInfo, tokens.ErrTxWithWrongStatus
	}

	if !txres.TransactionWithMetaData.MetaData.DeliveredAmount.IsPositive() {
		return swapInfo, tokens.ErrTxWithNoPayment
	}

	asset := txres.TransactionWithMetaData.MetaData.DeliveredAmount.Asset().String()
	if asset != "XRP" {
		return swapInfo, tokens.ErrNativeIsZero
	}

	payment, ok := txres.TransactionWithMetaData.Transaction.(*data.Payment)
	if !ok || payment.GetTransactionType() != data.PAYMENT {
		log.Printf("Not a payment transaction")
		return swapInfo, fmt.Errorf("not a payment transaction")
	}

	txRecipient := payment.Destination.String()
	routerContract := b.GetRouterContract("")
	if !common.IsEqualIgnoreCase(txRecipient, routerContract) {
		return swapInfo, tokens.ErrTxWithWrongReceiver
	}

	if success := parseGasSwapTxMemo(routerContract, swapInfo, payment.Memos); success != nil {
		log.Info("wrong memos", "memos", common.ToJSONString(payment.Memos, false))
		return swapInfo, tokens.ErrWrongBindAddress
	}

	swapInfo.From = routerContract // From
	swapInfo.Value = tokens.ToBits(txres.TransactionWithMetaData.MetaData.DeliveredAmount.Value.String(), 6)

	gasSwapInfo := swapInfo.GasSwapInfo
	srcCurrencyInfo, err := tokens.GetCurrencyInfo(swapInfo.FromChainID)
	if err != nil {
		return swapInfo, err
	}
	destCurrencyInfo, err := tokens.GetCurrencyInfo(swapInfo.ToChainID)
	if err != nil {
		return swapInfo, err
	}
	gasSwapInfo.SrcCurrencyPrice = srcCurrencyInfo.Price
	gasSwapInfo.DestCurrencyPrice = destCurrencyInfo.Price
	gasSwapInfo.SrcCurrencyDecimal = srcCurrencyInfo.Decimal
	gasSwapInfo.DestCurrencyDecimal = destCurrencyInfo.Decimal

	if !allowUnstable {
		log.Info("verify swapin pass",
			"asset", asset, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}
	return swapInfo, nil
}

func (b *Bridge) buildGasSwapTxArg(args *tokens.BuildTxArgs) (err error) {
	srcCurrencyPrice := args.GasSwapInfo.SrcCurrencyPrice
	destCurrencyPrice := args.GasSwapInfo.DestCurrencyPrice

	srcPrice := new(big.Float).SetInt(srcCurrencyPrice)
	destPrice := new(big.Float).SetInt(destCurrencyPrice)
	srcFloat, _ := srcPrice.Float64()
	destFloat, _ := destPrice.Float64()

	priceRate := big.NewFloat(srcFloat / destFloat)
	value := new(big.Float).SetInt(args.OriginValue)
	amount, _ := value.Mul(value, priceRate).Int64()

	input := []byte(args.SwapID)
	args.Input = (*hexutil.Bytes)(&input)
	if !b.IsValidAddress(args.Bind) {
		return tokens.ErrWrongBindAddress
	}
	args.To = args.Bind             // to
	args.Value = big.NewInt(amount) // swapValue

	log.Warn("buildGasSwapTx", "srcPrice", srcCurrencyPrice, "destPrice", destCurrencyPrice, "priceRate", priceRate, "amount", amount)

	return nil
}

func parseGasSwapTxMemo(routerContract string, swapInfo *tokens.SwapTxInfo, memos data.Memos) error {
	for _, memo := range memos {
		memoStr := strings.TrimSpace(string(memo.Memo.MemoData.Bytes()))
		parts := strings.Split(memoStr, ":")
		if len(parts) < 2 {
			continue
		}
		bindStr := parts[0]
		toChainIDStr := parts[1]
		biToChainID, err := common.GetBigIntFromStr(toChainIDStr)
		if err != nil {
			continue
		}
		dstBridge := router.GetBridgeByChainID(toChainIDStr)
		if dstBridge == nil {
			continue
		}
		if dstBridge.IsValidAddress(bindStr) {
			if common.IsEqualIgnoreCase(bindStr, routerContract) {
				return tokens.ErrTxWithWrongReceipt
			}
			swapInfo.Bind = bindStr          // Bind
			swapInfo.ToChainID = biToChainID // ToChainID
			return nil
		}
	}
	return tokens.ErrTxMemo
}

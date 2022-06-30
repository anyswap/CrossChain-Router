package eth

import (
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

func (b *Bridge) registerGasSwapTx(txHash string, logIndex int) ([]*tokens.SwapTxInfo, []error) {
	swapInfo, err := b.verifyGasSwapTx(txHash, logIndex, true)
	return []*tokens.SwapTxInfo{swapInfo}, []error{err}
}

func (b *Bridge) parseGasSwapTxMemo(swapInfo *tokens.SwapTxInfo, payload *hexutil.Bytes) error {
	memoHex, err := hexutil.Decode(payload.String())
	if err != nil {
		return err
	}
	memo := strings.Split(string(memoHex), ":")

	if len(memo) != 2 {
		return tokens.ErrTxMemo
	}

	bind := memo[0]
	toChainID, err := common.GetBigIntFromStr(memo[1])
	if err != nil {
		return err
	}
	swapInfo.ToChainID = toChainID
	toBridge := router.GetBridgeByChainID(toChainID.String())
	if toBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	if !toBridge.IsValidAddress(bind) || common.IsEqualIgnoreCase(bind, b.GetRouterContract("")) {
		return tokens.ErrTxWithWrongReceiver
	}
	swapInfo.Bind = bind

	gasSwapInfo := swapInfo.GasSwapInfo
	srcCurrencyPrice, err := tokens.GetNativePrice(swapInfo.FromChainID)
	if err != nil {
		return err
	}
	destCurrencyPrice, err := tokens.GetNativePrice(swapInfo.ToChainID)
	if err != nil {
		return err
	}
	gasSwapInfo.SrcCurrencyPrice = srcCurrencyPrice
	gasSwapInfo.DestCurrencyPrice = destCurrencyPrice
	return nil
}

func (b *Bridge) checkGasSwapTx(swapInfo *tokens.SwapTxInfo, allowUnstable bool) (err error) {
	txStatus, err := b.GetTransactionStatus(swapInfo.Hash)
	if err != nil {
		log.Error("get tx receipt failed", "hash", swapInfo.Hash, "err", err)
		return err
	}
	if txStatus == nil || txStatus.BlockHeight == 0 {
		return tokens.ErrTxNotFound
	}
	if txStatus.BlockHeight < b.ChainConfig.InitialHeight {
		return tokens.ErrTxBeforeInitialHeight
	}

	swapInfo.Height = txStatus.BlockHeight  // Height
	swapInfo.Timestamp = txStatus.BlockTime // Timestamp

	if !allowUnstable && txStatus.Confirmations < b.ChainConfig.Confirmations {
		return tokens.ErrTxNotStable
	}

	receipt, ok := txStatus.Receipt.(*types.RPCTxReceipt)
	if !ok {
		return tokens.ErrTxWithWrongReceipt
	}

	routerContract := b.GetRouterContract("")

	if receipt.Recipient == nil || receipt.Recipient.LowerHex() != strings.ToLower(routerContract) {
		return tokens.ErrTxWithWrongReceiver
	}

	swapInfo.From = strings.ToLower(routerContract) // From
	if *receipt.From == (common.Address{}) {
		return tokens.ErrTxWithWrongSender
	}

	return nil
}

func (b *Bridge) getGasSwapTxInput(swapInfo *tokens.SwapTxInfo, allowUnstable bool) (*hexutil.Bytes, error) {
	txInfo, err := b.GetTransactionByHash(swapInfo.Hash)
	if err != nil {
		log.Error("get tx info failed", "hash", swapInfo.Hash, "err", err)
		return nil, err
	}
	if txInfo.Payload == nil {
		return nil, tokens.ErrTxMemo
	}

	amount := (*big.Int)(txInfo.Amount)
	if amount.Cmp(big.NewInt(0)) == 0 {
		return nil, tokens.ErrNativeIsZero
	}
	swapInfo.Value = amount
	log.Warn("getGasSwapTxInput", "value", swapInfo.Value)
	return txInfo.Payload, nil
}

func (b *Bridge) verifyGasSwapTx(txHash string, _ int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{GasSwapInfo: &tokens.GasSwapInfo{}}}
	swapInfo.SwapType = tokens.GasSwapType  // SwapType
	swapInfo.Hash = strings.ToLower(txHash) // Hash
	swapInfo.LogIndex = 0                   // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID()

	err := b.checkGasSwapTx(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	payload, err := b.getGasSwapTxInput(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	err = b.parseGasSwapTxMemo(swapInfo, payload)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		ctx := []interface{}{
			"identifier", params.GetIdentifier(),
			"from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value,
			"txid", txHash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp,
			"fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID,
		}
		log.Info("verify router swap tx stable pass", ctx...)
	}

	return swapInfo, nil
}

func (b *Bridge) buildGasSwapTxInput(args *tokens.BuildTxArgs) (err error) {
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
	args.To = args.Bind // to
	//todo decimals calc
	args.Value = big.NewInt(amount) // swapValue

	log.Warn("buildGasSwapTxInput", "srcPrice", srcCurrencyPrice, "destPrice", destCurrencyPrice, "priceRate", priceRate, "amount", amount)

	return nil
}

package btc

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/btcsuite/btcwallet/wallet/txsizes"
)

var (
	retryCount       = 3
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second

	UnlockMemoPrefix     = "SWAPTX:"
	retryInterval        = 3 * time.Second
	cfgEstimateFeeBlocks = 6
	cfgPlusFeePercentage uint64
	cfgMinRelayFeePerKb  int64 = 2000
	cfgMaxRelayFeePerKb  int64 = 500000
)

// BuildRawTransaction build raw tx
//nolint:gocyclo // ok
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.Input != nil {
		return nil, fmt.Errorf("forbid build raw swap tx with input data")
	}
	if args.From == "" {
		return nil, fmt.Errorf("forbid empty sender")
	}
	routerMPC, getMpcErr := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if getMpcErr != nil {
		return nil, getMpcErr
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}
	switch args.SwapType {
	case tokens.ERC20SwapType:
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, b.GetChainConfig().ChainID)
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return nil, tokens.ErrMissTokenConfig
	}

	token := b.GetTokenConfig(multichainToken)
	if token == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return nil, err
	}
	args.SwapValue = amount // SwapValue

	txOuts, err := b.getTxOutputs(receiver, amount, UnlockMemoPrefix+args.SwapID)
	if err != nil {
		return nil, err
	}

	relayFee, errf := b.getRelayFeePerKb()
	if errf != nil {
		return nil, errf
	}
	relayFeePerKb := btcAmountType(relayFee)

	inputSource := func(target btcAmountType) (total btcAmountType, inputs []*wireTxInType, inputValues []btcAmountType, scripts [][]byte, err error) {
		return b.selectUtxos(args.From, target)
	}

	changeSource := func() ([]byte, error) {
		return b.GetPayToAddrScript(args.From)
	}
	rawTx, err = b.NewUnsignedTransaction(txOuts, relayFeePerKb, inputSource, changeSource)
	if err != nil {
		return nil, err
	}

	return rawTx, nil
}

func (b *Bridge) NewUnsignedTransaction(outputs []*wire.TxOut, relayFeePerKb btcutil.Amount,
	fetchInputs txauthor.InputSource, fetchChange txauthor.ChangeSource) (*txauthor.AuthoredTx, error) {

	targetAmount := txauthor.SumOutputValues(outputs)
	estimatedSize := txsizes.EstimateVirtualSize(0, 1, 0, outputs, true)
	targetFee := txrules.FeeForSerializeSize(relayFeePerKb, estimatedSize)

	for {
		inputAmount, inputs, inputValues, scripts, err := fetchInputs(targetAmount + targetFee)
		if err != nil {
			return nil, err
		}
		if inputAmount < targetAmount+targetFee {
			return nil, errors.New("insufficient funds available to construct transaction")
		}

		// We count the types of inputs, which we'll use to estimate
		// the vsize of the transaction.
		var nested, p2wpkh, p2pkh int
		for _, pkScript := range scripts {
			switch {
			// If this is a p2sh output, we assume this is a
			// nested P2WKH.
			case txscript.IsPayToScriptHash(pkScript):
				nested++
			case txscript.IsPayToWitnessPubKeyHash(pkScript):
				p2wpkh++
			default:
				p2pkh++
			}
		}

		maxSignedSize := txsizes.EstimateVirtualSize(p2pkh, p2wpkh,
			nested, outputs, true)
		maxRequiredFee := txrules.FeeForSerializeSize(relayFeePerKb, maxSignedSize)
		remainingAmount := inputAmount - targetAmount
		if remainingAmount < maxRequiredFee {
			targetFee = maxRequiredFee
			continue
		}

		unsignedTransaction := &wire.MsgTx{
			Version:  wire.TxVersion,
			TxIn:     inputs,
			TxOut:    outputs,
			LockTime: 0,
		}
		changeIndex := -1
		changeAmount := inputAmount - targetAmount - maxRequiredFee
		if changeAmount != 0 && !txrules.IsDustAmount(changeAmount,
			txsizes.P2WPKHPkScriptSize, relayFeePerKb) {
			changeScript, err := fetchChange()
			if err != nil {
				return nil, err
			}
			// if len(changeScript) > txsizes.P2WPKHPkScriptSize {
			// 	return nil, errors.New("fee estimation requires change " +
			// 		"scripts no larger than P2WPKH output scripts")
			// }
			change := wire.NewTxOut(int64(changeAmount), changeScript)
			l := len(outputs)
			unsignedTransaction.TxOut = append(outputs[:l:l], change)
			changeIndex = l
		}

		return &txauthor.AuthoredTx{
			Tx:              unsignedTransaction,
			PrevScripts:     scripts,
			PrevInputValues: inputValues,
			TotalInput:      inputAmount,
			ChangeIndex:     changeIndex,
		}, nil
	}
}

func (b *Bridge) findUxtosWithRetry(from string) (utxos []*ElectUtxo, err error) {
	for i := 0; i < retryCount; i++ {
		utxos, err = b.FindUtxos(from)
		if err == nil {
			break
		}
		time.Sleep(retryInterval)
	}
	return utxos, err
}

func (b *Bridge) selectUtxos(from string, target btcAmountType) (total btcAmountType, inputs []*wireTxInType, inputValues []btcAmountType, scripts [][]byte, err error) {
	p2pkhScript, err := b.GetPayToAddrScript(from)
	if err != nil {
		return 0, nil, nil, nil, err
	}

	utxos, err := b.findUxtosWithRetry(from)
	if err != nil {
		return 0, nil, nil, nil, err
	}

	var (
		tx      *ElectTx
		success bool
	)

	for _, utxo := range utxos {
		value := btcAmountType(*utxo.Value)
		if !isValidValue(value) {
			continue
		}
		tx, err = b.getTransactionByHashWithRetry(*utxo.Txid)
		if err != nil {
			continue
		}
		if *utxo.Vout >= uint32(len(tx.Vout)) {
			continue
		}
		output := tx.Vout[*utxo.Vout]
		if *output.ScriptpubkeyType != p2pkhType {
			continue
		}
		if output.ScriptpubkeyAddress == nil || *output.ScriptpubkeyAddress != from {
			continue
		}

		txIn, errf := b.NewTxIn(*utxo.Txid, *utxo.Vout, p2pkhScript)
		if errf != nil {
			continue
		}

		total += value
		inputs = append(inputs, txIn)
		inputValues = append(inputValues, value)
		scripts = append(scripts, p2pkhScript)

		if total >= target {
			success = true
			break
		}
	}

	if !success {
		err = fmt.Errorf("not enough balance, total %v < target %v", total, target)
		return 0, nil, nil, nil, err
	}

	return total, inputs, inputValues, scripts, nil
}

func (b *Bridge) getRelayFeePerKb() (estimateFee int64, err error) {
	for i := 0; i < retryCount; i++ {
		estimateFee, err = b.EstimateFeePerKb(cfgEstimateFeeBlocks)
		if err == nil {
			break
		}
		time.Sleep(retryInterval)
	}
	if err != nil {
		log.Warn("estimate smart fee failed", "err", err)
		return 0, err
	}
	if cfgPlusFeePercentage > 0 {
		estimateFee += estimateFee * int64(cfgPlusFeePercentage) / 100
	}
	if estimateFee > cfgMaxRelayFeePerKb {
		estimateFee = cfgMaxRelayFeePerKb
	} else if estimateFee < cfgMinRelayFeePerKb {
		estimateFee = cfgMinRelayFeePerKb
	}
	return estimateFee, nil
}

func (b *Bridge) getTxOutputs(to string, amount *big.Int, memo string) (txOuts []*wireTxOutType, err error) {
	if amount != nil {
		err = b.addPayToAddrOutput(&txOuts, to, amount.Int64())
		if err != nil {
			return nil, err
		}
	}

	if memo != "" {
		err = b.addMemoOutput(&txOuts, memo)
		if err != nil {
			return nil, err
		}
	}

	return txOuts, err
}

func (b *Bridge) addMemoOutput(txOuts *[]*wireTxOutType, memo string) error {
	if memo == "" {
		return nil
	}
	nullScript, err := b.NullDataScript(memo)
	if err != nil {
		return err
	}
	*txOuts = append(*txOuts, b.NewTxOut(0, nullScript))
	return nil
}

func (b *Bridge) addPayToAddrOutput(txOuts *[]*wireTxOutType, to string, amount int64) error {
	if amount <= 0 {
		return nil
	}
	pkscript, err := b.GetPayToAddrScript(to)
	if err != nil {
		return err
	}
	*txOuts = append(*txOuts, b.NewTxOut(amount, pkscript))
	return nil
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount *big.Int, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
	if !b.IsValidAddress(receiver) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("swapout to invalid receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	if fromBridge == nil {
		return receiver, amount, tokens.ErrNoBridgeForChainID
	}
	fromTokenCfg := fromBridge.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", erc20SwapInfo.Token)
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	amount = tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	return receiver, amount, err
}

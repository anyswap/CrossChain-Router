package aptos

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	maxFee              string = "2000"
	defaultGasUnitPrice string = "100"
	timeout_seconds     int64  = 600
)

// BuildRawTransaction impl
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.SwapType != tokens.GasSwapType {
		return nil, tokens.ErrSwapTypeNotSupported
	}

	if args.From == "" {
		return nil, errors.New("forbid empty sender")
	}
	routerMPC, err := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}

	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return nil, tokens.ErrMissMPCPublicKey
	}

	err = b.buildGasSwapTxArg(args)
	if err != nil {
		return nil, err
	}

	err = b.SetExtraArgs(args)
	if err != nil {
		return nil, err
	}

	tx, err := b.BuildTransferTransaction(args)
	if err != nil {
		return nil, err
	}

	log.Warnf("=========:%+v payload:%+v mpcPubkey:%+v", tx, tx.Payload, mpcPubkey)
	// Simulated transactions must have a non-valid signature
	err = b.SimulateTranscation(tx, mpcPubkey)
	if err != nil {
		return nil, fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, err)
	}

	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "bind", args.Bind, "nonce", tx.SequenceNumber,
		"gasPrice", tx.GasUnitPrice, "gasCurrency", tx.GasCurrencyCode,
		"originValue", args.OriginValue, "swapValue", args.SwapValue,
		"replaceNum", args.GetReplaceNum(),
	}
	log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()), ctx...)
	return tx, nil
}

func (b *Bridge) SetExtraArgs(args *tokens.BuildTxArgs) error {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	extra := args.Extra
	extra.EthExtra = nil // clear this which may be set in replace job
	if extra.Sequence == nil {
		sequence, err := b.getAccountNonce(args)
		if err != nil {
			if strings.Contains(err.Error(), "AptosError:") {
				return fmt.Errorf("%w %v", tokens.ErrBuildTxErrorAndDelay, err)
			}
			return err
		}
		extra.Sequence = sequence
	}
	if extra.BlockHash == nil {
		// 10 min
		expiration := strconv.FormatInt(time.Now().Unix()+timeout_seconds, 10)
		extra.BlockHash = &expiration
	}
	if extra.Gas == nil {
		gas, err := strconv.ParseUint(b.getGasPrice(), 10, 64)
		if err != nil {
			return err
		}
		extra.Gas = &gas
	}
	if extra.Fee == nil {
		extra.Fee = &maxFee
	}
	log.Info("Build tx with extra args", "extra", extra)
	return nil
}

// BuildSwapinTransferTransaction build swapin transfer tx
func (b *Bridge) BuildTransferTransaction(args *tokens.BuildTxArgs) (*Transaction, error) {
	tx := &Transaction{
		Sender:                  args.From,
		SequenceNumber:          strconv.FormatUint(*args.Extra.Sequence, 10),
		MaxGasAmount:            *args.Extra.Fee,
		GasUnitPrice:            strconv.FormatUint(*args.Extra.Gas, 10),
		ExpirationTimestampSecs: *args.Extra.BlockHash,
		Payload: &TransactionPayload{
			Type:          SCRIPT_FUNCTION_PAYLOAD,
			Function:      NATIVE_TRANSFER,
			TypeArguments: []string{},
			Arguments:     []interface{}{args.To, args.Value.String()},
		},
	}
	return tx, nil
}

func (b *Bridge) getGasPrice() string {
	estimateGasPrice, err := b.EstimateGasPrice()
	if err == nil {
		log.Debugln("estimateGasPrice", "GasPrice", estimateGasPrice.GasPrice)
		return strconv.Itoa(estimateGasPrice.GasPrice)
	} else {
		log.Debugln("estimateGasPrice", "GasPrice", defaultGasUnitPrice)
		return defaultGasUnitPrice
	}
}

func (b *Bridge) getAccountNonce(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	if params.IsParallelSwapEnabled() {
		nonce, err = b.AllocateNonce(args)
		return &nonce, err
	}

	res, err := b.GetPoolNonce(args.From, "")
	if err != nil {
		return nil, err
	}

	nonce = b.AdjustNonce(args.From, res)
	return &nonce, nil
}

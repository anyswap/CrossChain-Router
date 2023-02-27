package tron

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"

	tronaddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second
)

// BuildRawTransaction build raw tx
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
	routerMPC, err := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}

	getOrInitTronExtra(args)

	switch args.SwapType {
	case tokens.ERC20SwapType:
		err = b.buildERC20SwapTxInput(args)
	case tokens.NFTSwapType:
		err = b.buildNFTSwapTxInput(args)
	case tokens.AnyCallSwapType:
		err = b.buildAnyCallSwapTxInput(args)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	if err != nil {
		return nil, err
	}

	return b.buildTx(args)
}

var SwapinFeeLimit int64 = 300000000 // 300 TRX

func (b *Bridge) buildTx(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	extra := args.Extra
	if extra.RawTx != nil {
		var tx core.Transaction
		err = proto.Unmarshal(extra.RawTx, &tx)
		if err != nil {
			return nil, err
		}
		err = b.verifyRawTransaction(&tx, args)
		if err != nil {
			return nil, err
		}
		return &tx, nil
	}

	var parameter string
	if args.Input != nil {
		parameter = hex.EncodeToString(*args.Input)
	}

	rawTx, err = b.BuildTriggerConstantContractTx(args.From, args.To, args.Selector, parameter, SwapinFeeLimit)

	ctx := []interface{}{
		"identifier", args.Identifier, "swapID", args.SwapID,
		"fromChainID", args.FromChainID, "toChainID", args.ToChainID,
		"from", args.From, "to", args.To, "bind", args.Bind,
		"replaceNum", args.GetReplaceNum(),
		"selector", strings.Split(args.Selector, "(")[0],
		"feeLimit", SwapinFeeLimit,
	}
	switch {
	case args.ERC20SwapInfo != nil:
		ctx = append(ctx,
			"originValue", args.OriginValue,
			"swapValue", args.SwapValue,
			"tokenID", args.ERC20SwapInfo.TokenID)
	case args.NFTSwapInfo != nil:
		ctx = append(ctx,
			"tokenID", args.NFTSwapInfo.TokenID,
			"ids", args.NFTSwapInfo.IDs,
			"amounts", args.NFTSwapInfo.Amounts,
			"batch", args.NFTSwapInfo.Batch)
	}
	if err != nil {
		ctx = append(ctx, "err", err)
	}
	log.Info(fmt.Sprintf("build %s raw tx", args.SwapType.String()), ctx...)
	if err != nil {
		return nil, err
	}
	txmsg, err := proto.Marshal(rawTx.(*core.Transaction))
	if err != nil {
		return nil, err
	}
	extra.RawTx = txmsg

	return rawTx, nil
}

func getOrInitTronExtra(args *tokens.BuildTxArgs) *tokens.AllExtras {
	if args.Extra == nil {
		args.Extra = &tokens.AllExtras{}
	}
	return args.Extra
}

func (b *Bridge) verifyRawTransaction(tx *core.Transaction, args *tokens.BuildTxArgs) error {
	if args.Input == nil {
		return fmt.Errorf("tx input is nil")
	}

	feeLimit := tx.GetRawData().GetFeeLimit()
	if feeLimit != SwapinFeeLimit {
		log.Error("tx fee limit mismatch", "have", feeLimit, "want", SwapinFeeLimit)
		return fmt.Errorf("tx fee limit mismatch")
	}

	contract, err := getTriggerSmartContract(tx)
	if err != nil {
		return err
	}

	tokenID := args.GetTokenID()
	txRecipient := tronaddress.Address(contract.ContractAddress).String()
	checkReceiver, err := router.GetTokenRouterContract(tokenID, b.ChainConfig.ChainID)
	if err != nil {
		return err
	}

	if !strings.EqualFold(txRecipient, checkReceiver) {
		return fmt.Errorf("tx receiver mismatch")
	}

	data := contract.GetData()
	selector := common.Keccak256Hash([]byte(args.Selector)).Bytes()[:4]
	input := append(selector, (*args.Input)...)
	if !bytes.Equal(data, input) {
		log.Error("tx data mismatch", "have", hex.EncodeToString(data), "want", hex.EncodeToString(input))
		return fmt.Errorf("tx data mismatch")
	}

	return nil
}

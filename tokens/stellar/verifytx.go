package stellar

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/protocols/horizon/operations"
	"github.com/stellar/go/txnbuild"
)

var errTxResultType = errors.New("tx type is not horizon.Transaction")

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if len(msgHashes) < 1 {
		return fmt.Errorf("must provide msg hash")
	}

	tx, ok := rawTx.(*txnbuild.Transaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}

	txMsg, err := network.HashTransactionInEnvelope(tx.ToXDR(), b.NetworkStr)
	if err != nil {
		return err
	}

	signContent := common.ToHex(txMsg[:])

	if !strings.EqualFold(signContent, msgHashes[0]) {
		return fmt.Errorf("msg hash not match, recover: %v, claiming: %v", signContent, msgHashes[0])
	}

	return nil
}

// VerifyTransaction impl
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifySwapoutTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

//nolint:gocyclo,funlen // ok
func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	tx, err := b.GetTransaction(txHash)
	if err != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return nil, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*hProtocol.Transaction)
	if !ok {
		return nil, errTxResultType
	}

	if !allowUnstable {
		h, errf := b.GetLatestBlockNumber()
		if errf != nil {
			return nil, errf
		}

		if h < uint64(txres.Ledger)+b.GetChainConfig().Confirmations {
			return nil, tokens.ErrTxNotStable
		}
		if h < b.ChainConfig.InitialHeight {
			return nil, tokens.ErrTxBeforeInitialHeight
		}
	}

	// Check tx status
	if !txres.Successful {
		return nil, tokens.ErrTxWithWrongStatus
	}

	opts, err := b.GetOperations(txHash)
	if err != nil {
		return nil, err
	}
	opt, ok := opts[logIndex].(operations.Payment)
	if !ok || opt.GetType() != "payment" || !opt.TransactionSuccessful {
		return nil, fmt.Errorf("not a payment transaction")
	}

	return b.buildSwapInfoFromOperation(txres, &opt, logIndex)
}

func parseSwapMemos(swapInfo *tokens.SwapTxInfo, memoStr string) bool {
	if memoStr == "" {
		return false
	}
	memobytes, _ := base64.StdEncoding.DecodeString(memoStr)
	addrLen := int(memobytes[0:1][0])
	addEnd := 1 + addrLen
	bindStr := hex.EncodeToString(memobytes[1:addEnd])

	toChainIDStr := hex.EncodeToString(memobytes[addEnd:])
	bigToChainID, err := common.GetBigIntFromStr(toChainIDStr)
	if err != nil {
		return false
	}
	dstBridge := router.GetBridgeByChainID(bigToChainID.String())
	if dstBridge == nil {
		return false
	}
	if dstBridge.IsValidAddress(bindStr) {
		swapInfo.Bind = bindStr           // Bind
		swapInfo.ToChainID = bigToChainID // ToChainID
		return true
	}
	return false
}

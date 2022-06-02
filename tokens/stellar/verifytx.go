package stellar

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/protocols/horizon/operations"
)

var errTxResultType = errors.New("tx type is not horizon.Transaction")

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	if len(msgHashes) < 1 {
		return fmt.Errorf("must provide msg hash")
	}
	tx, ok := rawTx.(hProtocol.Transaction)
	if !ok {
		return fmt.Errorf("stellar tx type error")
	}
	fmt.Println(tx)
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
	swapInfo := &tokens.SwapTxInfo{}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = strings.ToLower(txHash)           // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	tx, err := b.GetTransaction(txHash)
	if err != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", err)
		return swapInfo, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*hProtocol.Transaction)
	if !ok {
		return swapInfo, errTxResultType
	}

	if !allowUnstable {
		h, errf := b.GetLatestBlockNumber()
		if errf != nil {
			return swapInfo, errf
		}

		if h < uint64(txres.Ledger)+b.GetChainConfig().Confirmations {
			return swapInfo, tokens.ErrTxNotStable
		}
		if h < b.ChainConfig.InitialHeight {
			return swapInfo, tokens.ErrTxBeforeInitialHeight
		}
	}

	// Check tx status
	if !txres.Successful {
		return swapInfo, tokens.ErrTxWithWrongStatus
	}

	opts, err := b.GetOperations(txHash)
	if err != nil {
		return swapInfo, err
	}
	opt, ok := opts[logIndex].(operations.Payment)
	if !ok || opt.GetType() != "payment" {
		return swapInfo, fmt.Errorf("not a payment transaction")
	}

	assetKey := convertTokenID(&opt)
	token := b.GetTokenConfig(assetKey)
	if token == nil {
		return swapInfo, tokens.ErrMissTokenConfig
	}

	txRecipient := opt.To
	// special usage, stellar has no router contract, and use deposit methods
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

	amt := tokens.ToBits(opt.Amount, token.Decimals)
	if amt.Cmp(big.NewInt(0)) <= 0 {
		return swapInfo, tokens.ErrTxWithWrongValue
	}

	swapInfo.To = depositAddress // To
	swapInfo.From = opt.From     // From
	swapInfo.Value = amt
	return swapInfo, nil
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

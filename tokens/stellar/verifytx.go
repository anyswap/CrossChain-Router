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
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
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
	if logIndex >= len(opts) {
		return nil, tokens.ErrLogIndexOutOfRange
	}
	opt := getPaymentOperation(opts[logIndex])
	if opt == nil {
		return nil, fmt.Errorf("not a payment transaction")
	}

	return b.buildSwapInfoFromOperation(txres, opt, logIndex)
}

// memo format:
// three parts: | 0 (bindAddrBytesLen) | 1...pos (bindAddr) | pos+1... (tochainID) |
// memo[0] (=len): bind address bytes length
// memo[1:len+1]: bind address bytes
// memo[len+1:]:  tochainId big.Int bytes
func checkSwapMemos(swapInfo *tokens.SwapTxInfo, memoStr string) bool {
	if memoStr == "" {
		return false
	}
	bindStr, bigToChainID := DecodeMemos(memoStr)
	if bigToChainID == nil || bindStr == "" {
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

func DecodeMemos(memoStr string) (string, *big.Int) {
	memobytes, err := base64.StdEncoding.DecodeString(memoStr)
	if err != nil || len(memobytes) == 0 {
		return "", nil
	}
	addrLen := int(memobytes[0])
	addEnd := 1 + addrLen
	if len(memobytes) < addEnd+1 {
		return "", nil
	}
	bindStr := common.ToHex(memobytes[1:addEnd])
	bigToChainID := new(big.Int).SetBytes(memobytes[addEnd:])
	return bindStr, bigToChainID
}

func EncodeMemo(chainId *big.Int, bindAddr string) (*txnbuild.MemoHash, error) {
	if common.HasHexPrefix(bindAddr) {
		bindAddr = bindAddr[2:]
	}
	b, err := hex.DecodeString(bindAddr)
	if err != nil {
		return nil, err
	}
	c := chainId.Bytes()
	if len(b)+len(c) > 31 {
		return nil, fmt.Errorf("memo too long,chainID %s addr %s", chainId.String(), bindAddr)
	}
	rtn := new(txnbuild.MemoHash)
	rtn[0] = byte(len(b))
	for i := 0; i < len(b); i++ {
		rtn[i+1] = b[i]
	}
	for i := 0; i < len(c); i++ {
		rtn[32-len(c)+i] = c[i]
	}
	return rtn, nil
}

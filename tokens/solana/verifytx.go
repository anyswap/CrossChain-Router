package solana

import (
	"bytes"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
	bin "github.com/streamingfast/binary"
)

var (
	// Program log: SwapoutBurn 0xdce8e16a5b685b7713436b4adf4ffd66bd0387d8 6qieHYFuMqTF7jbxXk2Wu3kZZD8rKMDTPEhe9ki4G4bK 1000000000 666
	// Program log: SwapoutTransfer 0xdce8e16a5b685b7713436b4adf4ffd66bd0387d8 6qieHYFuMqTF7jbxXk2Wu3kZZD8rKMDTPEhe9ki4G4bK 1000000000 666
	// Program log: SwapoutNative 0xdce8e16a5b685b7713436b4adf4ffd66bd0387d8 native 1000000000 666
	swapoutLogPattern         = regexp.MustCompile(`^Program log: (Swapout\w+) (\w+) (\w+) (\d+) (\d+)$`)
	firstProgramInvokePattern = regexp.MustCompile(`^Program (\w+) invoke \[1]`)
	swapoutInstructionPrefix  = "Program log: Instruction: Swapout"
	swapoutLogPrefix          = "Program log: Swapout"
)

// VerifyMsgHash impl
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}

	if len(msgHashes) != 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	msgHash := msgHashes[0]

	buf := new(bytes.Buffer)
	if err = bin.NewEncoder(buf).Encode(tx.Message); err != nil {
		return fmt.Errorf("unable to encode message for verifying: %w", err)
	}
	msgContent := buf.Bytes()

	if common.ToHex(msgContent) != msgHash {
		log.Trace("message hash mismatch", "want", msgHash, "have", common.ToHex(msgContent))
		return tokens.ErrMsgHashMismatch
	}
	return nil
}

// VerifyTransaction impl
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType // SwapType
	swapInfo.Hash = txHash                   // Hash
	swapInfo.LogIndex = args.LogIndex        // LogIndex

	allowUnstable := args.AllowUnstable
	txm, err := b.getTransactionMeta(swapInfo, allowUnstable)
	if err != nil {
		return swapInfo, err
	}

	err = b.verifySwapoutLogs(swapInfo, txm.LogMessages)
	if err != nil {
		return swapInfo, err
	}

	err = b.checkTokenSwapInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify router swap tx stable pass", "identifier", params.GetIdentifier(),
			"from", swapInfo.From, "to", swapInfo.To, "bind", swapInfo.Bind, "value", swapInfo.Value,
			"txid", txHash, "logIndex", swapInfo.LogIndex, "height", swapInfo.Height, "timestamp", swapInfo.Timestamp,
			"fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID,
			"token", swapInfo.ERC20SwapInfo.Token,
			"tokenID", swapInfo.ERC20SwapInfo.TokenID,
			"forUnderlying", swapInfo.ERC20SwapInfo.ForUnderlying)
	}

	return swapInfo, nil
}

func (b *Bridge) verifySwapoutLogs(swapInfo *tokens.SwapTxInfo, logMessages []string) error {
	logIndex := swapInfo.LogIndex
	// `6` here is determined by our concrete log pattern
	if logIndex+6 > len(logMessages) {
		return tokens.ErrLogIndexOutOfRange
	}

	routerProgramID := b.ChainConfig.RouterContract
	invokeStartPrefix := fmt.Sprintf("Program %s invoke [", routerProgramID)
	invokeSuccess := fmt.Sprintf("Program %s success", routerProgramID)

	matchTxTo := firstProgramInvokePattern.FindStringSubmatch(logMessages[0])
	if len(matchTxTo) != 2 {
		return tokens.ErrTxWithWrongContract
	}
	swapInfo.TxTo = matchTxTo[1]

	if !params.AllowCallByContract() &&
		!common.IsEqualIgnoreCase(swapInfo.TxTo, b.ChainConfig.RouterContract) &&
		!params.IsInCallByContractWhitelist(b.ChainConfig.ChainID, swapInfo.TxTo) {
		return tokens.ErrTxWithWrongContract
	}

	if !strings.HasPrefix(logMessages[logIndex], invokeStartPrefix) ||
		!strings.HasPrefix(logMessages[logIndex+1], swapoutInstructionPrefix) {
		return tokens.ErrSwapoutLogNotFound
	}
	swapInfo.To = routerProgramID

	swapoutLog := ""
	foundInvokeSuccess := false
	logMsgSlice := logMessages[logIndex+1:]
	for i, msg := range logMsgSlice {
		if strings.HasPrefix(msg, swapoutLogPrefix) &&
			i+2 < len(logMsgSlice) && logMsgSlice[i+2] == invokeSuccess {
			swapoutLog = msg
			foundInvokeSuccess = true
			break
		}
		if msg == invokeSuccess {
			foundInvokeSuccess = true
			break
		}
		// prevent possible reentrance
		if strings.HasPrefix(msg, invokeStartPrefix) {
			break
		}
	}
	if swapoutLog == "" {
		return tokens.ErrSwapoutLogNotFound
	}
	if !foundInvokeSuccess {
		return tokens.ErrTxWithWrongStatus
	}

	matches := swapoutLogPattern.FindStringSubmatch(swapoutLog)
	if len(matches) != 6 {
		return tokens.ErrSwapoutLogNotFound
	}

	erc20SwapInfo := swapInfo.ERC20SwapInfo

	swapoutType := matches[1]
	switch swapoutType {
	case "SwapoutBurn":
	case "SwapoutNative":
	case "SwapoutTransfer":
	default:
		return tokens.ErrUnknownSwapoutType
	}

	// matches parts is (to, mint, amount, to_chainid)
	swapInfo.Bind = matches[2]
	erc20SwapInfo.Token = matches[3]
	value, err := common.GetUint64FromStr(matches[4])
	if err != nil {
		return err
	}
	swapInfo.Value = new(big.Int).SetUint64(value)
	swapInfo.FromChainID = b.ChainConfig.GetChainID()
	toChainID, err := common.GetUint64FromStr(matches[5])
	if err != nil {
		return err
	}
	swapInfo.ToChainID = new(big.Int).SetUint64(toChainID)

	tokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	erc20SwapInfo.TokenID = tokenCfg.TokenID

	return nil
}

func (b *Bridge) checkTokenSwapInfo(swapInfo *tokens.SwapTxInfo) error {
	if swapInfo.FromChainID.String() != b.ChainConfig.ChainID {
		log.Error("router swap tx with mismatched fromChainID", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "fromChainID", swapInfo.FromChainID, "toChainID", swapInfo.ToChainID, "chainID", b.ChainConfig.ChainID)
		return tokens.ErrFromChainIDMismatch
	}
	erc20SwapInfo := swapInfo.ERC20SwapInfo
	fromTokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil || erc20SwapInfo.TokenID == "" {
		return tokens.ErrMissTokenConfig
	}
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, swapInfo.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", swapInfo.ToChainID)
		return tokens.ErrMissTokenConfig
	}
	toBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if toBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	toTokenCfg := toBridge.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		log.Warn("get token config failed", "chainID", swapInfo.ToChainID, "token", multichainToken)
		return tokens.ErrMissTokenConfig
	}
	if !tokens.CheckTokenSwapValue(swapInfo, fromTokenCfg.Decimals, toTokenCfg.Decimals) {
		return tokens.ErrTxWithWrongValue
	}
	dstBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if dstBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	if !dstBridge.IsValidAddress(swapInfo.Bind) {
		log.Warn("wrong bind address", "txid", swapInfo.Hash, "logIndex", swapInfo.LogIndex, "bind", swapInfo.Bind, "toChainID", swapInfo.ToChainID)
		return tokens.ErrWrongBindAddress
	}
	return nil
}

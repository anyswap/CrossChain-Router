package aptos

import (
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	tx, ok := rawTx.(*Transaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	signingMessage, err := b.GetSigningMessage(tx)
	if err != nil {
		return fmt.Errorf("unable to encode message for signing: %w", err)
	}
	if !strings.EqualFold(*signingMessage, msgHashes[0]) {
		log.Trace("message hash mismatch", "want", *signingMessage, "have", msgHashes[0])
		return tokens.ErrMsgHashMismatch
	}
	return nil
}

// VerifyTransaction impl
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.GasSwapType:
		return b.verifyGasSwapTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

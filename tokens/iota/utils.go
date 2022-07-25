package iota

import (
	"encoding/hex"

	"github.com/anyswap/CrossChain-Router/v3/log"
)

func ConvertMessageID(txHash string) ([32]byte, error) {
	var msgID [32]byte
	if messageID, err := hex.DecodeString(txHash); err != nil {
		log.Warn("decode message id error", "err", err)
		return msgID, err
	} else {
		copy(msgID[:], messageID)
		return msgID, nil
	}
}

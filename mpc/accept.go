package mpc

import (
	"encoding/json"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

// DoAcceptSign accept sign
func (c *Config) DoAcceptSign(keyID, agreeResult string, msgHash, msgContext []string) (string, error) {
	nonce := uint64(0)
	data := AcceptData{
		TxType:     "ACCEPTSIGN",
		Key:        keyID,
		Accept:     agreeResult,
		MsgHash:    msgHash,
		MsgContext: msgContext,
		TimeStamp:  common.NowMilliStr(),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	rawTX, err := BuildMPCRawTx(nonce, payload, c.defaultMPCNode.keyWrapper)
	if err != nil {
		return "", err
	}
	return c.AcceptSign(rawTX)
}

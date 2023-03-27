package iota

import (
	"encoding/hex"

	"github.com/anyswap/CrossChain-Router/v3/log"
	iotago "github.com/iotaledger/iota.go/v2"
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


func ConvertStringToAddress(edAddr string) *iotago.Ed25519Address {
	if eddr, err := iotago.ParseEd25519AddressFromHexString(edAddr); err == nil {
		return eddr
	}
	return nil
}

func ConvertPubKeyToAddr(addrPubKey string) *iotago.Ed25519Address {
	if publicKey, err := hex.DecodeString(addrPubKey); err != nil {
		return nil
	} else {
		eddr := iotago.AddressFromEd25519PubKey(publicKey)
		return &eddr
	}
}

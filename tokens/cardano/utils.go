package cardano

import (
	"crypto/ed25519"
	"encoding/hex"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
)

func StringToPrivateKey(priv string) (*ed25519.PrivateKey, error) {
	if data, err := hex.DecodeString(priv); err != nil {
		return nil, err
	} else {
		log.Info("StringToPrivateKey", "data", data)
		ed25519PriKey := ed25519.NewKeyFromSeed(data)
		return &ed25519PriKey, nil
	}
}

func CmdArgsVerify(args ...string) bool {
	for _, arg := range args {
		if strings.Contains(arg, " ") {
			return false
		}
	}
	return true
}

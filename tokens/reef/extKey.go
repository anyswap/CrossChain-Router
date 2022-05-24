package reef

import (
	"fmt"
	"strconv"
	"strings"
	substratetypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

type ExtKey struct {
    BlockHash string
    ExtIdx int
}

func ExtKeyFromRaw(raw string) (extKey ExtKey) {
    sl := strings.Split(string(raw), "-")
    if len(sl) != 2 {
		return extKey
	}
    extKey.BlockHash = sl[0]
    extKey.ExtIdx, _ = strconv.Atoi(sl[1])
    return
}

func (extKey ExtKey) String() string {
	return strings.ToLower(extKey.BlockHash) + "-" + strconv.Itoa(extKey.ExtIdx)
}

func (b *Bridge) StorageKey(extKey ExtKey) string {
	var metadata substratetypes.Metadata
	raw :=b.GetMetadata()
	err := substratetypes.DecodeFromHex(*raw, &metadata)
	if err != nil {
		return ""
	}
	key, err := substratetypes.CreateStorageKey(&metadata, "System", "Events", nil)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%#x\n", key)
}
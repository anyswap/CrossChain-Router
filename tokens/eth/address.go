package eth

import (
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

// IsValidAddress check address
func (b *Bridge) IsValidAddress(address string) bool {
	if !common.IsHexAddress(address) {
		return false
	}
	if b.DontCheckAddressMixedCase {
		return true
	}
	unprefixedHex, ok, hasUpperChar := common.GetUnprefixedHex(address)
	if hasUpperChar {
		// valid checksum
		if unprefixedHex != common.HexToAddress(address).Hex()[2:] {
			return false
		}
	}
	return ok
}

// IsContractAddress is contract address
func (b *Bridge) IsContractAddress(address string) (bool, error) {
	var code []byte
	var err error
	for i := 0; i < retryRPCCount; i++ {
		code, err = b.GetCode(address)
		if err == nil {
			return len(code) > 1, nil // unexpect RSK getCode return 0x00
		}
		time.Sleep(retryRPCInterval)
	}
	return false, err
}

package solana

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

// IsValidAddress impl check address
func (b *Bridge) IsValidAddress(address string) bool {
	_, err := types.PublicKeyFromBase58(address)
	return err == nil
}

// VerifyMPCPubKey verify mpc address and public key is matching
func VerifyMPCPubKey(mpcAddress, mpcPubkey string) error {
	pubAddr := types.PublicKeyFromBytes(common.FromHex(mpcPubkey))
	if pubAddr.String() != mpcAddress {
		return fmt.Errorf("mpc address %v and public key address %v is not match", mpcAddress, pubAddr.String())
	}
	return nil
}

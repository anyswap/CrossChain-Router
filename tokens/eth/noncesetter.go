package eth

import (
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
)

// NonceSetterBase base nonce setter
type NonceSetterBase struct {
	SwapNonce map[string]uint64 // key is sender address
}

// NewNonceSetterBase new base nonce setter
func NewNonceSetterBase() *NonceSetterBase {
	return &NonceSetterBase{
		SwapNonce: make(map[string]uint64),
	}
}

// GetSwapNonce get current swap nonce
func (b *Bridge) GetSwapNonce(address string) uint64 {
	account := strings.ToLower(address)
	return b.SwapNonce[account]
}

// AdjustNonce adjust account nonce (eth like chain)
func (b *Bridge) AdjustNonce(address string, value uint64) (nonce uint64) {
	account := strings.ToLower(address)
	if b.SwapNonce[account] > value {
		nonce = b.SwapNonce[account]
	} else {
		nonce = value
	}
	return nonce
}

// InitSwapNonce init swap nonce
func (b *Bridge) InitSwapNonce(address string, nonce uint64) {
	account := strings.ToLower(address)
	for i := 0; i < retryRPCCount; i++ {
		pendingNonce, err := b.GetPoolNonce(account, "pending")
		if err == nil {
			if pendingNonce > nonce {
				log.Warn("init swap nonce with onchain account nonce", "dbNonce", nonce, "accountNonce", pendingNonce)
				nonce = pendingNonce
			}
			break
		}
		if i+1 == retryRPCCount {
			log.Warn("init swap nonce get account nonce failed", "account", account, "err", err)
		}
		time.Sleep(retryRPCInterval)
	}
	b.SwapNonce[account] = nonce
	log.Info("init swap nonce success", "account", account, "nonce", nonce)
}

// SetNonce set account nonce (eth like chain)
func (b *Bridge) SetNonce(address string, value uint64) {
	account := strings.ToLower(address)
	if b.SwapNonce[account] < value {
		b.SwapNonce[account] = value
	}
}

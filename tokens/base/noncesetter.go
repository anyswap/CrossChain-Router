package base

import (
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second
)

// NonceSetterBase base nonce setter
type NonceSetterBase struct {
	*tokens.CrossChainBridgeBase
	swapNonce map[string]*uint64 // key is sender address

	swapNonceLock sync.RWMutex
}

// NewNonceSetterBase new base nonce setter
func NewNonceSetterBase() *NonceSetterBase {
	return &NonceSetterBase{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
		swapNonce:            make(map[string]*uint64),
	}
}

// GetSwapNonce get current swap nonce
func (b *NonceSetterBase) GetSwapNonce(address string) uint64 {
	b.swapNonceLock.RLock()
	defer b.swapNonceLock.RUnlock()

	account := strings.ToLower(address)
	if nonceptr, exist := b.swapNonce[account]; exist {
		return *nonceptr
	}
	return 0
}

// AdjustNonce adjust account nonce (eth like chain)
func (b *NonceSetterBase) AdjustNonce(address string, value uint64) (nonce uint64) {
	b.swapNonceLock.Lock()
	defer b.swapNonceLock.Unlock()

	account := strings.ToLower(address)

	var old uint64
	if nonceptr, exist := b.swapNonce[account]; exist {
		old = *nonceptr
	}
	if old > value {
		nonce = old
	} else {
		nonce = value
	}
	log.Info("adjust nonce", "chainID", b.ChainConfig.ChainID, "account", account, "old", value, "new", nonce)
	if value > 2*old+1000 {
		log.Warn("forbid adjust nonce (too big)", b.ChainConfig.ChainID, "account", account, "old", old, "new", value)
		return old
	}
	return nonce
}

// InitSwapNonce init swap nonce
func (b *NonceSetterBase) InitSwapNonce(br tokens.NonceSetter, address string, nonce uint64) {
	b.swapNonceLock.Lock()
	defer b.swapNonceLock.Unlock()

	dbNexNonce := nonce
	for i := 0; i < retryRPCCount; i++ {
		pendingNonce, err := br.GetPoolNonce(address, "pending")
		if err == nil {
			if pendingNonce > nonce {
				log.Warn("init swap nonce with onchain account nonce", "chainID", b.ChainConfig.ChainID, "dbNonce", nonce, "accountNonce", pendingNonce)
				nonce = pendingNonce
			}
			break
		}
		if i+1 == retryRPCCount {
			log.Warn("init swap nonce get account nonce failed", "chainID", b.ChainConfig.ChainID, "account", address, "err", err)
		}
		time.Sleep(retryRPCInterval)
	}
	b.swapNonce[strings.ToLower(address)] = &nonce
	log.Info("init swap nonce success", "chainID", b.ChainConfig.ChainID, "account", address, "dbNexNonce", dbNexNonce, "nonce", nonce)
}

// SetNonce set account nonce (eth like chain)
func (b *NonceSetterBase) SetNonce(address string, value uint64) {
	b.swapNonceLock.Lock()
	defer b.swapNonceLock.Unlock()

	account := strings.ToLower(address)
	var old uint64
	if nonceptr, exist := b.swapNonce[account]; exist {
		old = *nonceptr
		if old < value {
			*nonceptr = value
		}
	} else {
		b.swapNonce[account] = &value
	}
	if old < value {
		var chainID string
		if b.ChainConfig != nil {
			chainID = b.ChainConfig.ChainID
		}
		log.Info("set next nonce", "chainID", chainID, "account", account, "old", old, "new", value)
	}
}

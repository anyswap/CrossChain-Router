package base

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second

	recycleAckInterval = int64(300) // seconds

	errRecycleNotAcked = errors.New("recycle timestamp does not pass ack interval")
)

type recycleNonceRecord struct {
	nonce     uint64
	timestamp int64
}

// NonceSetterBase base nonce setter
type NonceSetterBase struct {
	*tokens.CrossChainBridgeBase
	swapNonce    map[string]*uint64             // key is sender address
	recycleNonce map[string]*recycleNonceRecord // key is sender address

	swapNonceLock        sync.RWMutex
	recycleSwapNonceLock sync.RWMutex
}

// NewNonceSetterBase new base nonce setter
func NewNonceSetterBase() *NonceSetterBase {
	return &NonceSetterBase{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
		swapNonce:            make(map[string]*uint64),
		recycleNonce:         make(map[string]*recycleNonceRecord),
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
	account := strings.ToLower(address)
	for i := 0; i < retryRPCCount; i++ {
		pendingNonce, err := br.GetPoolNonce(account, "pending")
		if err == nil {
			if pendingNonce > nonce {
				log.Warn("init swap nonce with onchain account nonce", "chainID", b.ChainConfig.ChainID, "dbNonce", nonce, "accountNonce", pendingNonce)
				nonce = pendingNonce
			}
			break
		}
		if i+1 == retryRPCCount {
			log.Warn("init swap nonce get account nonce failed", "chainID", b.ChainConfig.ChainID, "account", account, "err", err)
		}
		time.Sleep(retryRPCInterval)
	}
	b.swapNonce[account] = &nonce
	log.Info("init swap nonce success", "chainID", b.ChainConfig.ChainID, "account", account, "dbNexNonce", dbNexNonce, "nonce", nonce)
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
		log.Info("set next nonce", "chainID", b.ChainConfig.ChainID, "account", account, "old", old, "new", value)
	}
}

// AllocateNonce allocate nonce
func (b *NonceSetterBase) AllocateNonce(args *tokens.BuildTxArgs) (nonce uint64, err error) {
	if nonce, err = b.TryAllocateRecycleNonce(args, recycleAckInterval); err == nil {
		return nonce, nil
	}

	b.swapNonceLock.Lock()
	defer b.swapNonceLock.Unlock()

	account := strings.ToLower(args.From)
	allocNonce, exist := b.swapNonce[account]
	if !exist {
		initNonce := uint64(0)
		allocNonce = &initNonce
		b.swapNonce[account] = allocNonce
	}
	return mongodb.AllocateRouterSwapNonce(args, allocNonce, false)
}

// TryAllocateRecycleNonce try allocate recycle swap nonce
func (b *NonceSetterBase) TryAllocateRecycleNonce(args *tokens.BuildTxArgs, lifetime int64) (nonce uint64, err error) {
	b.recycleSwapNonceLock.RLock()
	defer b.recycleSwapNonceLock.RUnlock()

	account := strings.ToLower(args.From)
	rec, exist := b.recycleNonce[account]
	if !exist || time.Now().Unix()-rec.timestamp < lifetime {
		return 0, errRecycleNotAcked
	}
	return mongodb.AllocateRouterSwapNonce(args, &rec.nonce, true)
}

// RecycleSwapNonce recycle swap nonce
func (b *NonceSetterBase) RecycleSwapNonce(sender string, nonce uint64) {
	b.recycleSwapNonceLock.Lock()
	defer b.recycleSwapNonceLock.Unlock()

	account := strings.ToLower(sender)
	rec, exist := b.recycleNonce[account]
	if !exist {
		b.recycleNonce[account] = &recycleNonceRecord{
			nonce:     nonce,
			timestamp: time.Now().Unix(),
		}
	} else if rec.nonce == 0 || nonce < rec.nonce {
		rec.nonce = nonce
		rec.timestamp = time.Now().Unix()
	}
}

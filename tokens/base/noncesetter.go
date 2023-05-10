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

type nonceLocks struct {
	swapNonceLock        sync.RWMutex
	recycleSwapNonceLock sync.RWMutex
}

// NonceSetterBase base nonce setter
type NonceSetterBase struct {
	*tokens.CrossChainBridgeBase
	swapNonce    map[string]*uint64             // key is sender address
	recycleNonce map[string]*recycleNonceRecord // key is sender address

	locks map[string]*nonceLocks
}

// NewNonceSetterBase new base nonce setter
func NewNonceSetterBase() *NonceSetterBase {
	return &NonceSetterBase{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
		swapNonce:            make(map[string]*uint64),
		locks:                make(map[string]*nonceLocks),
	}
}

func (b *NonceSetterBase) GetSwapNonceLock(address string) *sync.RWMutex {
	account := strings.ToLower(address)
	lock, exist := b.locks[account]
	if !exist {
		lock = &nonceLocks{}
		b.locks[account] = lock
	}
	return &lock.swapNonceLock
}

func (b *NonceSetterBase) GetRecycleSwapNonceLock(address string) *sync.RWMutex {
	account := strings.ToLower(address)
	lock, exist := b.locks[account]
	if !exist {
		lock = &nonceLocks{}
		b.locks[account] = lock
	}
	return &lock.recycleSwapNonceLock
}

// GetSwapNonce get current swap nonce
func (b *NonceSetterBase) GetSwapNonce(address string) uint64 {
	account := strings.ToLower(address)
	swapNonceLock := b.GetSwapNonceLock(account)
	swapNonceLock.RLock()
	defer swapNonceLock.RUnlock()

	if nonceptr, exist := b.swapNonce[account]; exist {
		return *nonceptr
	}
	return 0
}

// AdjustNonce adjust account nonce (eth like chain)
func (b *NonceSetterBase) AdjustNonce(address string, value uint64) (nonce uint64) {
	account := strings.ToLower(address)
	swapNonceLock := b.GetSwapNonceLock(account)
	swapNonceLock.RLock()
	defer swapNonceLock.RUnlock()

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
	account := strings.ToLower(address)
	swapNonceLock := b.GetSwapNonceLock(account)
	swapNonceLock.RLock()
	defer swapNonceLock.RUnlock()

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
	b.swapNonce[account] = &nonce
	log.Info("init swap nonce success", "chainID", b.ChainConfig.ChainID, "account", address, "dbNexNonce", dbNexNonce, "nonce", nonce)
}

// SetNonce set account nonce (eth like chain)
func (b *NonceSetterBase) SetNonce(address string, value uint64) {
	account := strings.ToLower(address)
	swapNonceLock := b.GetSwapNonceLock(account)
	swapNonceLock.RLock()
	defer swapNonceLock.RUnlock()

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

// AllocateNonce allocate nonce
func (b *NonceSetterBase) AllocateNonce(args *tokens.BuildTxArgs) (nonce uint64, err error) {
	if nonce, err = b.TryAllocateRecycleNonce(args, recycleAckInterval); err == nil {
		return nonce, nil
	}

	account := strings.ToLower(args.From)
	swapNonceLock := b.GetSwapNonceLock(account)
	swapNonceLock.RLock()
	defer swapNonceLock.RUnlock()

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
	account := strings.ToLower(args.From)
	recycleSwapNonceLock := b.GetRecycleSwapNonceLock(account)
	recycleSwapNonceLock.RLock()
	defer recycleSwapNonceLock.RUnlock()

	rec, exist := b.recycleNonce[account]
	if !exist || time.Now().Unix()-rec.timestamp < lifetime {
		return 0, errRecycleNotAcked
	}
	return mongodb.AllocateRouterSwapNonce(args, &rec.nonce, true)
}

// RecycleSwapNonce recycle swap nonce
func (b *NonceSetterBase) RecycleSwapNonce(sender string, nonce uint64) {
	account := strings.ToLower(sender)
	recycleSwapNonceLock := b.GetRecycleSwapNonceLock(account)
	recycleSwapNonceLock.RLock()
	defer recycleSwapNonceLock.RUnlock()

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

package eth

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
	recycleSwapNonceLock sync.RWMutex
	recycleAckInterval   = int64(300) // seconds

	errRecycleNotAcked = errors.New("recycle timestamp does not pass ack interval")
)

type recycleNonceRecord struct {
	nonce     uint64
	timestamp int64
}

// NonceSetterBase base nonce setter
type NonceSetterBase struct {
	swapNonce    map[string]*uint64             // key is sender address
	recycleNonce map[string]*recycleNonceRecord // key is sender address
}

// NewNonceSetterBase new base nonce setter
func NewNonceSetterBase() *NonceSetterBase {
	return &NonceSetterBase{
		swapNonce:    make(map[string]*uint64),
		recycleNonce: make(map[string]*recycleNonceRecord),
	}
}

// GetSwapNonce get current swap nonce
func (b *Bridge) GetSwapNonce(address string) uint64 {
	account := strings.ToLower(address)
	if nonceptr, exist := b.swapNonce[account]; exist {
		return *nonceptr
	}
	return 0
}

// AdjustNonce adjust account nonce (eth like chain)
func (b *Bridge) AdjustNonce(address string, value uint64) (nonce uint64) {
	account := strings.ToLower(address)
	if nonceptr, exist := b.swapNonce[account]; exist && *nonceptr > value {
		nonce = *nonceptr
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
	b.swapNonce[account] = &nonce
	log.Info("init swap nonce success", "account", account, "nonce", nonce)
}

// SetNonce set account nonce (eth like chain)
func (b *Bridge) SetNonce(address string, value uint64) {
	account := strings.ToLower(address)
	if nonceptr, exist := b.swapNonce[account]; exist {
		if *nonceptr < value {
			*nonceptr = value
		}
	} else {
		b.swapNonce[account] = &value
	}
}

// AllocateNonce allocate nonce
func (b *Bridge) AllocateNonce(args *tokens.BuildTxArgs) (nonce uint64, err error) {
	if nonce, err = b.TryAllocateRecycleNonce(args, recycleAckInterval); err == nil {
		return nonce, nil
	}
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
func (b *Bridge) TryAllocateRecycleNonce(args *tokens.BuildTxArgs, lifetime int64) (nonce uint64, err error) {
	recycleSwapNonceLock.RLock()
	defer recycleSwapNonceLock.RUnlock()

	account := strings.ToLower(args.From)
	rec, exist := b.recycleNonce[account]
	if !exist || time.Now().Unix()-rec.timestamp < lifetime {
		return 0, errRecycleNotAcked
	}
	return mongodb.AllocateRouterSwapNonce(args, &rec.nonce, true)
}

// RecycleSwapNonce recycle swap nonce
func (b *Bridge) RecycleSwapNonce(sender string, nonce uint64) {
	recycleSwapNonceLock.Lock()
	defer recycleSwapNonceLock.Unlock()

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

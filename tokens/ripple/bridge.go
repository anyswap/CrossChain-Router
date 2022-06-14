package ripple

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/websockets"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.NonceSetter
	_ tokens.NonceSetter = &Bridge{}

	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once

	rpcRetryTimes    = 3
	rpcRetryInterval = 1 * time.Second
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
	devnetNetWork  = "devnet"
)

// Bridge block bridge inherit from btc bridge
type Bridge struct {
	*base.NonceSetterBase
	Remotes map[string]*websockets.Remote
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		NonceSetterBase: base.NewNonceSetterBase(),
	}
}

// SupportsChainID supports chainID
func SupportsChainID(chainID *big.Int) bool {
	supportedChainIDsInit.Do(func() {
		supportedChainIDs[GetStubChainID(mainnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(testnetNetWork).String()] = true
		supportedChainIDs[GetStubChainID(devnetNetWork).String()] = true
	})
	return supportedChainIDs[chainID.String()]
}

// GetStubChainID get stub chainID
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("XRP"))
	switch network {
	case mainnetNetWork:
	case testnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(1))
	case devnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(2))
	default:
		log.Fatalf("unknown network %v", network)
	}
	stubChainID.Mod(stubChainID, tokens.StubChainIDBase)
	stubChainID.Add(stubChainID, tokens.StubChainIDBase)
	return stubChainID
}

// SetRPCRetryTimes set rpc retry times (used in cmd tools)
func SetRPCRetryTimes(times int) {
	rpcRetryTimes = times
}

// GetLatestBlockNumber gets latest block number
// For ripple, GetLatestBlockNumber returns current ledger version
func (b *Bridge) GetLatestBlockNumber() (num uint64, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		for url, r := range b.Remotes {
			if !r.IsConnected {
				log.Trace("[GetLatestBlockNumber] connection is closed", "url", url)
				continue
			}
			resp, err1 := r.Ledger(nil, false)
			if err1 != nil || resp == nil {
				err = err1
				log.Warn("Try get latest block number failed", "error", err1)
				continue
			}
			num = uint64(resp.Ledger.LedgerSequence)
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetLatestBlockNumberOf gets latest block number from single api
// For ripple, GetLatestBlockNumberOf returns current ledger version
func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	var err error
	r, exist := b.Remotes[apiAddress]
	if !exist {
		r, err = websockets.NewRemote(apiAddress)
		if err != nil {
			log.Warn("Cannot connect to remote", "error", err)
			return 0, err
		}
		defer r.Close()
	}
	resp, err := r.Ledger(nil, false)
	if err != nil || resp == nil {
		return 0, err
	}
	return uint64(resp.Ledger.LedgerSequence), nil
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	txhash256, err := data.NewHash256(txHash)
	if err != nil {
		return
	}
	for i := 0; i < rpcRetryTimes; i++ {
		for url, r := range b.Remotes {
			if !r.IsConnected {
				log.Trace("[GetTransaction] connection is closed", "url", url)
				continue
			}
			resp, err1 := r.Tx(*txhash256)
			if err1 != nil || resp == nil {
				log.Warn("Try get transaction failed", "error", err1)
				err = err1
				continue
			}
			tx = resp
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)
	tx, err := b.GetTransaction(txHash)
	if err != nil {
		return nil, err
	}

	txres, ok := tx.(*websockets.TxResult)
	if !ok {
		// unexpected
		log.Warn("Ripple GetTransactionStatus", "error", errTxResultType)
		return nil, errTxResultType
	}

	// Check tx status
	if !txres.TransactionWithMetaData.MetaData.TransactionResult.Success() {
		log.Warn("Ripple tx status is not success", "result", txres.TransactionWithMetaData.MetaData.TransactionResult)
		return nil, tokens.ErrTxWithWrongStatus
	}

	status.Receipt = nil
	inledger := txres.LedgerSequence
	status.BlockHeight = uint64(inledger)

	if latest, err := b.GetLatestBlockNumber(); err == nil && latest > uint64(inledger) {
		status.Confirmations = latest - uint64(inledger)
	}
	return
}

// GetBlockHash gets block hash
func (b *Bridge) GetBlockHash(num uint64) (hash string, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		for url, r := range b.Remotes {
			if !r.IsConnected {
				log.Trace("[GetBlockHash] connection is closed", "url", url)
				continue
			}
			resp, err1 := r.Ledger(num, false)
			if err1 != nil || resp == nil {
				err = err1
				log.Warn("Try get block hash failed", "error", err1)
				continue
			}
			hash = resp.Ledger.Hash.String()
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetBlockTxids gets glock txids
func (b *Bridge) GetBlockTxids(num uint64) (txs []string, err error) {
	txs = make([]string, 0)
	for i := 0; i < rpcRetryTimes; i++ {
		for url, r := range b.Remotes {
			if !r.IsConnected {
				log.Trace("[GetBlockTxids] connection is closed", "url", url)
				continue
			}
			resp, err1 := r.Ledger(num, true)
			if err1 != nil || resp == nil {
				err = err1
				log.Warn("Try get block tx ids failed", "error", err1)
				continue
			}
			for _, tx := range resp.Ledger.Transactions {
				txs = append(txs, tx.GetBase().Hash.String())
			}
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetBalance gets balance
func (b *Bridge) GetBalance(accountAddress string) (*big.Int, error) {
	acct, err := b.GetAccount(accountAddress)
	if err != nil || acct == nil {
		log.Warn("get balance failed", "account", accountAddress, "err", err)
		return nil, err
	}
	bal := big.NewInt(acct.AccountData.Balance.Drops())
	return bal, nil
}

// GetAccount returns account
func (b *Bridge) GetAccount(address string) (acct *websockets.AccountInfoResult, err error) {
	account, err := data.NewAccountFromAddress(address)
	if err != nil {
		return
	}
	for i := 0; i < rpcRetryTimes; i++ {
		for url, r := range b.Remotes {
			if !r.IsConnected {
				log.Trace("[GetAccount] connection is closed", "url", url)
				continue
			}
			acct, err = r.AccountInfo(*account)
			if err != nil || acct == nil {
				continue
			}
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetAccountLine get account line
func (b *Bridge) GetAccountLine(currency, issuer, accountAddress string) (*data.AccountLine, error) {
	account, err := data.NewAccountFromAddress(accountAddress)
	if err != nil {
		return nil, err
	}
	var acclRes *websockets.AccountLinesResult
	for i := 0; i < rpcRetryTimes; i++ {
		for url, r := range b.Remotes {
			if !r.IsConnected {
				log.Trace("[GetAccountLine] connection is closed", "url", url)
				continue
			}
			acclRes, err = r.AccountLines(*account, nil)
			if err == nil && acclRes != nil {
				break
			}
		}
		time.Sleep(rpcRetryInterval)
	}
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(acclRes.Lines); i++ {
		accl := &acclRes.Lines[i]
		asset := accl.Asset()
		if asset.Currency == currency && asset.Issuer == issuer {
			return accl, nil
		}
	}
	return nil, fmt.Errorf("account line not found")
}

// GetFee get fee
func (b *Bridge) GetFee() (*websockets.FeeResult, error) {
	var feeRes *websockets.FeeResult
	var err error
	for i := 0; i < rpcRetryTimes; i++ {
		for url, r := range b.Remotes {
			if !r.IsConnected {
				log.Trace("[GetFee] connection is closed", "url", url)
				continue
			}
			feeRes, err = r.Fee()
			if err == nil && feeRes != nil {
				return feeRes, nil
			}
		}
		time.Sleep(rpcRetryInterval)
	}
	return nil, err
}

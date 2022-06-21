package ripple

import (
	"math/big"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
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

	wrapRPCQueryError = tokens.WrapRPCQueryError
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
	devnetNetWork  = "devnet"
)

// Bridge block bridge inherit from btc bridge
type Bridge struct {
	*base.NonceSetterBase
	RPCClientTimeout int
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		NonceSetterBase:  base.NewNonceSetterBase(),
		RPCClientTimeout: 60,
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
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	for _, url := range urls {
		num, err = b.GetLatestBlockNumberOf(url)
		if err == nil {
			return num, nil
		}
	}
	return 0, err
}

// GetLatestBlockNumberOf gets latest block number from single api
// For ripple, GetLatestBlockNumberOf returns current ledger version
func (b *Bridge) GetLatestBlockNumberOf(url string) (num uint64, err error) {
	rpcParams := map[string]interface{}{}
	for i := 0; i < rpcRetryTimes; i++ {
		var res *websockets.LedgerCurrentResult
		err = client.RPCPostWithTimeout(b.RPCClientTimeout, &res, url, "ledger_current", rpcParams)
		if err == nil && res != nil {
			return uint64(res.LedgerSequence), nil
		}
	}
	return 0, wrapRPCQueryError(err, "GetLatestBlockNumber")
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionByHash call eth_getTransactionByHash
func (b *Bridge) GetTransactionByHash(txHash string) (txRes *websockets.TxResult, err error) {
	rpcParams := map[string]interface{}{
		"transaction": txHash,
	}
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	for i := 0; i < rpcRetryTimes; i++ {
		for _, url := range urls {
			var res *websockets.TxResult
			err = client.RPCPostWithTimeout(b.RPCClientTimeout, &res, url, "tx", rpcParams)
			if err == nil && res != nil {
				return res, nil
			}
		}
		time.Sleep(rpcRetryInterval)
	}
	return nil, wrapRPCQueryError(err, "GetTransaction")
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
	return status, nil
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
func (b *Bridge) GetAccount(address string) (acctRes *websockets.AccountInfoResult, err error) {
	rpcParams := map[string]interface{}{
		"account":      address,
		"ledger_index": "current",
	}
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	for i := 0; i < rpcRetryTimes; i++ {
		for _, url := range urls {
			var res *websockets.AccountInfoResult
			err = client.RPCPostWithTimeout(b.RPCClientTimeout, &res, url, "account_info", rpcParams)
			if err == nil && res != nil {
				return res, nil
			}
		}
		time.Sleep(rpcRetryInterval)
	}
	return nil, wrapRPCQueryError(err, "GetAccount")
}

// GetAccountLine get account line
func (b *Bridge) GetAccountLine(currency, issuer, accountAddress string) (line *data.AccountLine, err error) {
	rpcParams := map[string]interface{}{
		"account":      accountAddress,
		"ledger_index": "validated",
	}
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	var acclRes *websockets.AccountLinesResult
OUT_LOOP:
	for i := 0; i < rpcRetryTimes; i++ {
		for _, url := range urls {
			var res *websockets.AccountLinesResult
			err = client.RPCPostWithTimeout(b.RPCClientTimeout, &res, url, "account_lines", rpcParams)
			if err == nil && res != nil {
				acclRes = res
				break OUT_LOOP
			}
		}
		time.Sleep(rpcRetryInterval)
	}
	if err != nil {
		log.Error("GetAccountLine rpc error", "err", err)
		return nil, err
	}
	for i := 0; i < len(acclRes.Lines); i++ {
		accl := &acclRes.Lines[i]
		asset := accl.Asset()
		if asset.Currency == currency && asset.Issuer == issuer {
			return accl, nil
		}
	}
	return nil, wrapRPCQueryError(err, "GetAccountLine", currency, issuer, accountAddress)
}

// GetFee get fee
func (b *Bridge) GetFee() (feeRes *websockets.FeeResult, err error) {
	rpcParams := map[string]interface{}{}
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	for i := 0; i < rpcRetryTimes; i++ {
		for _, url := range urls {
			var res *websockets.FeeResult
			err = client.RPCPostWithTimeout(b.RPCClientTimeout, &res, url, "fee", rpcParams)
			if err == nil && res != nil {
				return res, nil
			}
		}
		time.Sleep(rpcRetryInterval)
	}
	return nil, wrapRPCQueryError(err, "GetFee")
}

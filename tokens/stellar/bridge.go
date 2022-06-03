package stellar

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.NonceSetter
	_ tokens.NonceSetter = &Bridge{}

	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once
	networkChainIDs       = make(map[string]string)

	rpcQueryLimit    = uint(200)
	rpcRetryTimes    = 3
	rpcRetryInterval = 1 * time.Second
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
)

// Bridge block bridge inherit from btc bridge
type Bridge struct {
	*base.NonceSetterBase
	NetworkStr string
	Remotes    map[string]*horizonclient.Client
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge(chainID string) *Bridge {
	return &Bridge{
		NonceSetterBase: base.NewNonceSetterBase(),
		NetworkStr:      networkChainIDs[chainID],
		Remotes:         make(map[string]*horizonclient.Client),
	}
}

// SupportsChainID supports chainID
func SupportsChainID(chainID *big.Int) bool {
	supportedChainIDsInit.Do(func() {
		supportedChainIDs[GetStubChainID(mainnetNetWork).String()] = true
		networkChainIDs[GetStubChainID(mainnetNetWork).String()] = network.PublicNetworkPassphrase
		supportedChainIDs[GetStubChainID(testnetNetWork).String()] = true
		networkChainIDs[GetStubChainID(testnetNetWork).String()] = network.TestNetworkPassphrase
	})
	return supportedChainIDs[chainID.String()]
}

// GetStubChainID get stub chainID
func GetStubChainID(network string) *big.Int {
	stubChainID := new(big.Int).SetBytes([]byte("XLM"))
	switch network {
	case mainnetNetWork:
		stubChainID.Add(stubChainID, big.NewInt(1))
	case testnetNetWork:
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
// For stellar, GetLatestBlockNumber returns current ledger version
func (b *Bridge) GetLatestBlockNumber() (num uint64, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		for _, c := range b.Remotes {
			ledgerRequest := horizonclient.LedgerRequest{
				Order: horizonclient.OrderDesc,
				Limit: 1,
			}
			resp, err1 := c.Ledgers(ledgerRequest)
			if err1 != nil {
				err = err1
				log.Warn("Try get latest block number failed", "error", err1)
				continue
			}
			num = uint64(resp.Embedded.Records[0].Sequence)
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetLatestBlockNumberOf gets latest block number from single api
// For stellar, GetLatestBlockNumberOf returns current ledger version
func (b *Bridge) GetLatestBlockNumberOf(apiAddress string) (uint64, error) {
	var err error
	r, exist := b.Remotes[apiAddress]
	if !exist {
		r := horizonclient.DefaultPublicNetClient
		r.HorizonURL = apiAddress
		b.Remotes[apiAddress] = r
	}
	ledgerRequest := horizonclient.LedgerRequest{
		Order: horizonclient.OrderDesc,
		Limit: 1,
	}
	resp, err := r.Ledgers(ledgerRequest)
	if err != nil {
		return 0, err
	}
	return uint64(resp.Embedded.Records[0].Sequence), nil
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		for _, r := range b.Remotes {
			resp, err1 := r.TransactionDetail(txHash)
			if err1 != nil {
				log.Warn("Try get transaction failed", "error", err1)
				err = err1
				continue
			}
			tx = &resp
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

	relTx, ok := tx.(*hProtocol.Transaction)
	if !ok {
		// unexpected
		log.Warn("Stellar GetTransactionStatus", "error", errTxResultType)
		return nil, errTxResultType
	}

	// Check tx status
	if !relTx.Successful {
		log.Warn("Stellar tx status is not success", "result", relTx.ResultMetaXdr)
		return nil, tokens.ErrTxWithWrongStatus
	}

	status.Receipt = nil
	inledger := relTx.Ledger
	status.BlockHeight = uint64(inledger)
	// stellar use FBA which means need not have to wait for several ledgers to get confirmed.
	status.Confirmations = 0
	return
}

// GetBlockHash gets block hash
func (b *Bridge) GetBlockHash(num uint64) (hash string, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		for _, r := range b.Remotes {
			resp, err1 := r.LedgerDetail(uint32(num))
			if err1 != nil {
				err = err1
				log.Warn("Try get block hash failed", "error", err1)
				continue
			}
			hash = resp.Hash
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetBlockTxids gets glock txids
func (b *Bridge) GetBlockTxids(num uint64) (txs []string, err error) {
	for i := 0; i < rpcRetryTimes; i++ {
		txs = make([]string, 0)
		for _, r := range b.Remotes {
			request := horizonclient.TransactionRequest{
				ForLedger: uint(num),
				Limit:     rpcQueryLimit,
			}
			resp, err1 := r.Transactions(request)
			if err1 != nil {
				err = err1
				log.Warn("Try get block tx ids failed", "error", err1)
				continue
			}
			nextPage := true
			for nextPage {
				for _, tx := range resp.Embedded.Records {
					txs = append(txs, tx.Hash)
				}
				if len(resp.Embedded.Records) >= int(rpcQueryLimit) {
					resp, err1 = r.NextTransactionsPage(resp)
					if err1 != nil {
						err = err1
						log.Warn("Try get block tx ids failed", "error", err1)
						return
					}
				} else {
					nextPage = false
				}
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
	bal := big.NewInt(0)
	for _, asset := range acct.Balances {
		if asset.Type == "native" {
			f, err := strconv.ParseFloat(asset.Balance, 64)
			if err != nil {
				log.Warn("balance format error", "account", accountAddress, "err", asset.Balance)
			}
			bal = big.NewInt(int64(f))
			break
		}
	}
	return bal, err
}

func (b *Bridge) checkXMLBalanceEnough(acct *hProtocol.Account) bool {
	ok := false
	for _, asset := range acct.Balances {
		if asset.Type == "native" {
			f, err := strconv.ParseFloat(asset.Balance, 64)
			if err != nil || f < 1.0 {
				log.Error("stellar XML not enough", "account", acct.AccountID, "xml", asset.Balance)
			}
			ok = true
			break
		}
	}
	return ok
}

// GetAccount returns account
func (b *Bridge) GetAccount(address string) (acct *hProtocol.Account, err error) {
	destAccountRequest := horizonclient.AccountRequest{
		AccountID: address,
	}
	for i := 0; i < rpcRetryTimes; i++ {
		for _, r := range b.Remotes {
			resp, err1 := r.AccountDetail(destAccountRequest)
			if err1 != nil {
				continue
			}
			err = err1
			acct = &resp
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetAccount returns account
func (b *Bridge) GetAsset(code string, address string) (acct *hProtocol.AssetStat, err error) {
	request := horizonclient.AssetRequest{
		ForAssetCode:   code,
		ForAssetIssuer: address,
	}
	for i := 0; i < rpcRetryTimes; i++ {
		for _, r := range b.Remotes {
			resp, err1 := r.Assets(request)
			err = err1
			if err1 != nil {
				continue
			}
			if len(resp.Embedded.Records) <= 0 {
				err = errors.New("balance format error")
			}
			acct = &resp.Embedded.Records[0]
			return
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

// GetAccountLine get account line
func (b *Bridge) GetAccountLine(currency, issuer, accountAddress string) (string, error) {
	// account, err := data.NewAccountFromAddress(accountAddress)
	// if err != nil {
	// 	return nil, err
	// }
	// var acclRes *websockets.AccountLinesResult
	// for i := 0; i < rpcRetryTimes; i++ {
	// 	for _, r := range b.Remotes {
	// 		acclRes, err = r.AccountLines(*account, nil)
	// 		if err == nil && acclRes != nil {
	// 			break
	// 		}
	// 	}
	// 	time.Sleep(rpcRetryInterval)
	// }
	// if err != nil {
	// 	return nil, err
	// }
	// for i := 0; i < len(acclRes.Lines); i++ {
	// 	accl := &acclRes.Lines[i]
	// 	asset := accl.Asset()
	// 	if asset.Currency == currency && asset.Issuer == issuer {
	// 		return accl, nil
	// 	}
	// }
	return "", fmt.Errorf("account line not found")
}

// GetFee get fee
func (b *Bridge) GetFee() int {
	return txnbuild.MinBaseFee
}

func (b *Bridge) GetOperations(txHash string) (opts []interface{}, err error) {
	req := horizonclient.OperationRequest{
		ForTransaction: txHash,
		Limit:          rpcQueryLimit,
	}
	for i := 0; i < rpcRetryTimes; i++ {
		opts = make([]interface{}, 0)
		for _, r := range b.Remotes {
			resp, err1 := r.Operations(req)
			if err1 != nil {
				continue
			}
			nextPage := true
			for nextPage {
				for _, op := range resp.Embedded.Records {
					opts = append(opts, op)
				}
				if len(resp.Embedded.Records) >= int(rpcQueryLimit) {
					resp, err1 = r.NextOperationsPage(resp)
					if err1 != nil {
						err = err1
						log.Warn("Try get block tx ids failed", "error", err1)
						return nil, err
					}
				} else {
					nextPage = false
				}
			}
			return opts, nil
		}
		time.Sleep(rpcRetryInterval)
	}
	return
}

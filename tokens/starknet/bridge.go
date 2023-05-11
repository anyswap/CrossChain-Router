package starknet

import (
	"github.com/dontpanicdao/caigo/types"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet/rpcv02"
)

// ref: https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L1045
var addressPattern = "^0x(0|[a-fA-F1-9]{1}[a-fA-F0-9]{0,62})$"

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.NonceSetter
	_ tokens.NonceSetter = &Bridge{}

	supportedChainIDs     = make(map[string]bool)
	supportedChainIDsInit sync.Once
)

const (
	mpcAddrIdx = iota
	defaultAddrIdx
	privKeyIdx
)

const (
	mainnetNetWork = "mainnet"
	testnetNetWork = "testnet"
	devnetNetWork  = "devnet"
)

// Bridge inherits from base bridge
type Bridge struct {
	*base.NonceSetterBase
	RPCClientTimeout int

	defaultAccount *Account // account used for estimate gas fee
	mpcAccount     *Account // account used for sign txn, does not contain private key

	provider Provider
}

func (b *Bridge) GetPoolNonce(address, _ string) (uint64, error) {
	return b.provider.Nonce(address)
}

func (b *Bridge) GetTransaction(txHash string) (interface{}, error) {
	return b.provider.TransactionByHash(txHash)
}

func (b *Bridge) GetTransactionStatus(txHash string) (*tokens.TxStatus, error) {
	rawReceipt, err := b.provider.TransactionReceipt(txHash)
	if err != nil {
		return nil, tokens.ErrTxWithWrongReceipt
	}
	receipt, ok := rawReceipt.(rpcv02.InvokeTransactionReceipt)
	if !ok {
		return nil, tokens.ErrInvalidInvokeReceipt
	}

	status := tokens.TxStatus{
		Receipt:     receipt,
		BlockHeight: receipt.BlockNumber,
		BlockHash:   receipt.BlockHash.String(),
	}
	return &status, nil
}

func (b *Bridge) GetLatestBlockNumber() (num uint64, err error) {
	return b.provider.BlockNumber()
}

func (b *Bridge) GetLatestBlockNumberOf(string) (num uint64, err error) {
	return b.GetLatestBlockNumber()
}

func (b *Bridge) IsValidAddress(address string) bool {
	ok, _ := regexp.MatchString(addressPattern, address)
	return ok
}

func (b *Bridge) PubKeyToMpcPubKey(pubKey string) (string, error) {
	return pubKey, nil
}

func (b *Bridge) PublicKeyToAddress(pubKeyHex string) (string, error) {
	// CC: Starknet use account abstraction, an account address maps to a contract address, but not a public key.
	// In OpenZeppelin's implementation, account's public key is just a field variable of that account contract,
	// which is able to change. So there is no one-to-one relationship between an address and a public key.
	log.Warn("starknet uses account abstraction, public key does not map to an address")
	return pubKeyHex, nil
}

func (b *Bridge) WaitForTransaction(transactionHash types.Hash, pollInterval time.Duration) (types.TransactionState, error) {
	return b.provider.WaitForTransaction(transactionHash, pollInterval)
}

func (b *Bridge) InitAfterConfig() {
	var err error
	extra := strings.Split(b.ChainConfig.Extra, ":") // format: mpcAccount:defaultAccount:privateKey
	if len(extra) != 3 {
		log.Errorf("chainConfig extra error")
	}

	mpcAddr := extra[mpcAddrIdx]
	defaultAddr := extra[defaultAddrIdx]
	privKey := extra[privKeyIdx]

	defaultAccount, err := NewAccountWithPrivateKey(privKey, defaultAddr, b.ChainConfig.ChainID)
	if err != nil {
		log.Error("error creating starknet default account: ", err)
	}
	b.defaultAccount = defaultAccount

	mpcAccount, err := NewAccount(mpcAddr, b.ChainConfig.ChainID)
	if err != nil {
		log.Error("error creating starknet mpc account: ", err)
	}
	b.mpcAccount = mpcAccount
	//b.account.chainId = b.ChainConfig.ChainID
	for _, url := range b.GatewayConfig.AllGatewayURLs {
		provider, err := NewProvider(url, b.ChainConfig.ChainID)
		if err == nil {
			b.provider = provider
			return
		}
	}
	log.Error("error connecting to starknet rpc: ", err)
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
	stubChainID := new(big.Int).SetBytes([]byte("Starknet"))
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

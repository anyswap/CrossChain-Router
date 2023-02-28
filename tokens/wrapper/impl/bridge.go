package impl

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var errForbiddenMethod = errors.New("forbidden wrapper method")

// Bridge bridge
type Bridge struct {
	*params.WrapperConfig
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge(cfg *params.WrapperConfig) *Bridge {
	return &Bridge{
		WrapperConfig: cfg,
	}
}

func (b *Bridge) callService(result interface{}, method string, params ...interface{}) error {
	callMethod := "bridge." + method
	err := client.RPCPostWithTimeout(b.RPCTimeout, &result, b.RPCAddress, callMethod, params...)
	if err != nil {
		log.Error(fmt.Sprintf("call %v failed", method), "err", err)
	} else {
		log.Info(fmt.Sprintf("call %v success", method), "result", common.ToJSONString(result, false))
	}
	return err
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(chainID, routerContract, routerVersion string) (err error) {
	if routerContract == "" {
		return nil
	}
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)
	routerMPC, err := b.GetMPCAddress(routerContract)
	if err != nil {
		log.Warn("get router mpc address failed", "chainID", chainID, "routerContract", routerContract, "err", err)
		return err
	}
	if routerMPC == "" {
		log.Warn("get router mpc address return an empty address", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
		return fmt.Errorf("empty router mpc address of router contract %v on chain %v", routerContract, chainID)
	}
	log.Info("get router mpc address success", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)

	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC: routerMPC,
		},
	)

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID),
		"routerContract", routerContract, "routerMPC", routerMPC)

	return nil
}

// RegisterSwap register swap.
// used in `RegisterRouterSwap` server rpc.
func (b *Bridge) RegisterSwap(txHash string, args *tokens.RegisterArgs) ([]*tokens.SwapTxInfo, []error) {
	var result RegisterSwapResult
	err := b.callService(&result, "RegisterSwap", txHash, args)
	if err != nil {
		swapinfo := &tokens.SwapTxInfo{
			SwapType: args.SwapType,
			Hash:     txHash,
			LogIndex: args.LogIndex,
		}
		return []*tokens.SwapTxInfo{swapinfo}, []error{err}
	}
	return result.SwapTxInfos, result.Errs
}

// VerifyTransaction verify swap tx is valid and success on chain with needed confirmations.
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	var result tokens.SwapTxInfo
	err := b.callService(&result, "VerifyTransaction", txHash, args)
	return &result, err
}

// BuildRawTransaction build tx with specified args.
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	err = b.callService(&rawTx, "BuildRawTransaction", args)
	return rawTx, err
}

// VerifyMsgHash verify message hash is same.
// 'message hash' here is the real content (usually a hash) which will be signed.
// used in `accept` work for oracles to replay the same tx on destination chain.
// oracle will only accept a sign info if and only if the oracle can
// verify the tx and rebuild a tx and ensure the message hash is same.
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHash []string) error {
	var result interface{}
	return b.callService(&result, "VerifyMsgHash", rawTx, msgHash)
}

type SignTxResult struct {
	SignedTx interface{}
	TxHash   string
}

// MPCSignTransaction mpc sign tx.
func (b *Bridge) MPCSignTransaction(rawTx interface{}, args *tokens.BuildTxArgs) (signedTx interface{}, txHash string, err error) {
	var result SignTxResult
	err = b.callService(&result, "MPCSignTransaction", rawTx, args)
	return result.SignedTx, result.TxHash, err
}

// SendTransaction send signed raw tx.
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	err = b.callService(&txHash, "SendTransaction", signedTx)
	return txHash, err
}

// GetTransaction get tx by hash.
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return nil, errForbiddenMethod
}

// GetTransactionStatus get tx status by hash.
// get tx related infos like block height, confirmations, receipts etc.
// These infos is used to verify tx is acceptable.
// you can extend `TxStatus` if fields in it is not enough to do the checking.
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	err = b.callService(&status, "GetTransactionStatus", txHash)
	return status, err
}

// GetLatestBlockNumber get latest block number through gateway urls.
// used in `GetRouterSwap` server rpc.
func (b *Bridge) GetLatestBlockNumber() (number uint64, err error) {
	err = b.callService(&number, "GetLatestBlockNumber")
	return number, err
}

// GetLatestBlockNumberOf get latest block number of specified url.
// used in `AdjustGatewayOrder` function.
func (b *Bridge) GetLatestBlockNumberOf(url string) (number uint64, err error) {
	return 0, errForbiddenMethod
}

// GetBalance get balance is used for checking budgets to prevent DOS attacking
func (b *Bridge) GetBalance(account string) (balance *big.Int, err error) {
	err = b.callService(&balance, "GetBalance", account)
	return balance, err
}

// IsValidAddress check if given `address` is valid on this chain.
// prevent swap to an invalid `bind` address which will make assets loss.
func (b *Bridge) IsValidAddress(address string) bool {
	var result bool
	err := b.callService(&result, "IsValidAddress", address)
	return result && err == nil
}

// PublicKeyToAddress public key to address
func (b *Bridge) PublicKeyToAddress(pubKey string) (address string, err error) {
	err = b.callService(&address, "PublicKeyToAddress", pubKey)
	return address, err
}

// GetMPCAddress get mpc address of router contract
func (b *Bridge) GetMPCAddress(routerContract string) (address string, err error) {
	err = b.callService(&address, "GetMPCAddress", routerContract)
	return address, err
}

// GetPoolNonce get pool nonce
func (b *Bridge) GetPoolNonce(address, height string) (nonce uint64, err error) {
	err = b.callService(&nonce, "GetPoolNonce", address, height)
	return nonce, err
}

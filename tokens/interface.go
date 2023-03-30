// Package tokens defines the common interfaces and supported bridges in sub directories.
package tokens

import (
	"math/big"
)

// IMPCSign interface
type IMPCSign interface {
	VerifyMsgHash(rawTx interface{}, msgHash []string) error
	MPCSignTransaction(rawTx interface{}, args *BuildTxArgs) (signedTx interface{}, txHash string, err error)
}

// IBridgeConfg interface
// implemented by 'CrossChainBridgeBase'
type IBridgeConfg interface {
	GetGatewayConfig() *GatewayConfig
	GetChainConfig() *ChainConfig
	GetTokenConfig(tokenAddr string) *TokenConfig

	GetRouterContract(token string) string
	GetRouterVersion(token string) string

	SetChainConfig(chainCfg *ChainConfig)
	SetGatewayConfig(gatewayCfg *GatewayConfig)
	SetTokenConfig(token string, tokenCfg *TokenConfig)
}

// IBridge interface
type IBridge interface {
	IBridgeConfg
	IMPCSign

	InitRouterInfo(routerContract, routerVersion string) error
	InitAfterConfig()

	RegisterSwap(txHash string, args *RegisterArgs) ([]*SwapTxInfo, []error)
	VerifyTransaction(txHash string, ars *VerifyArgs) (*SwapTxInfo, error)
	BuildRawTransaction(args *BuildTxArgs) (rawTx interface{}, err error)
	SendTransaction(signedTx interface{}) (txHash string, err error)

	GetTransaction(txHash string) (interface{}, error)
	GetTransactionStatus(txHash string) (*TxStatus, error)
	GetLatestBlockNumber() (uint64, error)
	GetLatestBlockNumberOf(url string) (uint64, error)

	IsValidAddress(address string) bool
	PublicKeyToAddress(pubKeyHex string) (string, error)

	// GetBalance get balance is used for checking budgets
	// to prevent DOS attacking (used in any`call)
	GetBalance(account string) (*big.Int, error)

	CalcProofID(args *BuildTxArgs) (string, error)
	GenerateProof(proofID string, args *BuildTxArgs) (string, error)
}

// ISwapTrade interface
type ISwapTrade interface {
	GetPairFor(factory, token0, token1 string) (string, error)
}

// NonceSetter interface (for eth-like)
type NonceSetter interface {
	InitSwapNonce(br NonceSetter, address string, nonce uint64)

	// sequential
	GetPoolNonce(address, height string) (uint64, error)
	SetNonce(address string, value uint64)
	AdjustNonce(address string, value uint64) (nonce uint64)

	// parallel
	AllocateNonce(args *BuildTxArgs) (nonce uint64, err error)
	RecycleSwapNonce(sender string, nonce uint64)
}

type ReSwapable interface {
	SetTxTimeout(args *BuildTxArgs, txTimeout *uint64)
	GetCurrentThreshold() (*uint64, error)
	IsTxTimeout(txValue, currentValue *uint64) bool
	SetReswapMaxValueRate(rate uint64)
	SetTimeoutConfig(txTimeout uint64)
	GetTimeoutConfig() uint64
}

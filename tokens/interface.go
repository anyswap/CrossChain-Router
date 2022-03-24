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

	SetChainConfig(chainCfg *ChainConfig)
	SetGatewayConfig(gatewayCfg *GatewayConfig)
	SetTokenConfig(token string, tokenCfg *TokenConfig)
}

// IBridgeConfigLoader interface
type IBridgeConfigLoader interface {
	InitGatewayConfig(chainID *big.Int)
	InitChainConfig(chainID *big.Int)
	InitTokenConfig(tokenID string, chainID *big.Int)

	ReloadChainConfig(chainID *big.Int)
	ReloadTokenConfig(tokenID string, chainID *big.Int)
	RemoveTokenConfig(tokenAddr string)
}

// IBridge interface
type IBridge interface {
	IBridgeConfg
	IMPCSign

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

	// GetBalance get balance is used for checking budgets
	// to prevent DOS attacking (used in anycall)
	GetBalance(account string) (*big.Int, error)
}

// ISwapTrade interface
type ISwapTrade interface {
	GetPairFor(factory, token0, token1 string) (string, error)
}

// NonceSetter interface (for eth-like)
type NonceSetter interface {
	GetPoolNonce(address, height string) (uint64, error)
	RecycleSwapNonce(sender string, nonce uint64)
}

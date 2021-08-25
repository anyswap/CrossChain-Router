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
type IBridgeConfg interface {
	GetGatewayConfig() *GatewayConfig
	GetChainConfig() *ChainConfig
	GetTokenConfig(tokenAddr string) *TokenConfig

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

	RegisterSwap(txHash string, args *RegisterArgs) ([]*SwapTxInfo, []error)
	VerifyTransaction(txHash string, ars *VerifyArgs) (*SwapTxInfo, error)
	BuildRawTransaction(args *BuildTxArgs) (rawTx interface{}, err error)
	SendTransaction(signedTx interface{}) (txHash string, err error)

	GetTransaction(txHash string) (interface{}, error)
	GetTransactionStatus(txHash string) (*TxStatus, error)
	GetLatestBlockNumber() (uint64, error)
	GetLatestBlockNumberOf(url string) (uint64, error)

	IsValidAddress(address string) bool
}

// ISwapTrade interface
type ISwapTrade interface {
	GetPairFor(factory, token0, token1 string) (string, error)
}

// NonceSetter interface (for eth-like)
type NonceSetter interface {
	GetPoolNonce(address, height string) (uint64, error)
	SetNonce(pairID string, value uint64)
	AdjustNonce(pairID string, value uint64) (nonce uint64)
}

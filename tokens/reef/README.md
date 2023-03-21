# Reef 

## Reef Docs
https://docs.reef.io/docs/users/introduction/

- Metamask
  https://docs.reef.io/docs/users/extension/
- web3
  https://github.com/reef-defi/evm-provider.js
- Truffle/Hardhat
  https://github.com/reef-defi/hardhat-reef
- Remix IDE
  https://remix.reefscan.com/
- Etherscan
  https://reefscan.com/
- GraphQL
https://github.com/reef-defi/reef-explorer/tree/develop/db/hasura/metadata/databases/reefexplorer/tables

## Router Github
https://github.com/anyswap/CrossChain-Router/tree/feature/reef-substrate

## Router Contract
Reef support evm contract
https://github.com/anyswap/anyswap-v1-core


## Account Mechanism
```mermaid
graph TB
    begin(mpc public key)--> IsRemaining{"generate"}
    IsRemaining --sr25519-->reefAddr[ss58Address]--> sign(sign tx)
    IsRemaining --> |ecdsa|evmAddr[evmAddr]--> interaction(evm calls)
```
## Class Diagram
```mermaid
classDiagram

class IMPCSign  {
	VerifyMsgHash(rawTx interface, msgHash []string) error
	MPCSignTransaction(rawTx interface, args *BuildTxArgs) (signedTx interface, txHash string, err error)
}
class IBridgeConfg {
	GetGatewayConfig() *GatewayConfig
	GetChainConfig() *ChainConfig
	GetTokenConfig(tokenAddr string) *TokenConfig
	GetRouterContract(token string) string

	SetChainConfig(chainCfg *ChainConfig)
	SetGatewayConfig(gatewayCfg *GatewayConfig)
	SetTokenConfig(token string, tokenCfg *TokenConfig)
}
class NonceSetter {
	InitSwapNonce(br NonceSetter, address string, nonce uint64)

	GetPoolNonce(address, height string) (uint64, error)
	SetNonce(address string, value uint64)
	AdjustNonce(address string, value uint64) (nonce uint64)

	AllocateNonce(args *BuildTxArgs) (nonce uint64, err error)
	RecycleSwapNonce(sender string, nonce uint64)
}

class IBridge{
    InitRouterInfo(routerContract string) error
	InitAfterConfig()
	RegisterSwap(txHash string, args *RegisterArgs) ([]*SwapTxInfo, []error)
	VerifyTransaction(txHash string, ars *VerifyArgs) (*SwapTxInfo, error)
	BuildRawTransaction(args *BuildTxArgs) (rawTx interface, err error)
	SendTransaction(signedTx interface) (txHash string, err error)
	GetTransaction(txHash string) (interface, error)
	GetTransactionStatus(txHash string) (*TxStatus, error)
	GetLatestBlockNumber() (uint64, error)
	GetLatestBlockNumberOf(url string) (uint64, error)
	IsValidAddress(address string) bool
	PublicKeyToAddress(pubKeyHex string) (string, error)
	GetBalance(account string) (*big.Int, error)
}

class CustomConfig {
	RPCClientTimeout int
	DontCheckAddressMixedCase bool
}

class EvmContractBridge {
	CallContract(contract string, data hexutil.Bytes, blockNumber string) (string, error)
	VerifyMPCPubKey(mpcAddress, mpcPubkey string) error
	GetTransactionReceipt(txHash string) (receipt *types.RPCTxReceipt, err error)
	GetLatestBlockNumberOf(url string) (latest uint64, err error)
	GetTransactionByHash(txHash string) (tx *types.RPCTransaction, err error)
}

class NonceSetterBase {
	*tokens.CrossChainBridgeBase
	swapNonce    map[string]*uint64
	recycleNonce map[string]*recycleNonceRecord
	swapNonceLock        sync.RWMutex
	recycleSwapNonceLock sync.RWMutex
}

class EthBridge{
    Signer        types.Signer
	SignerChainID *big.Int
	latestGasPrice  *big.Int
	autoMaxGasPrice *big.Int
}

class ReefBridge{
    WS            []*WebSocket
	SubstrateAPIs []*gsrpc.SubstrateAPI
	MetaData      *types.Metadata
}

    IMPCSign *-- IBridge
    IBridgeConfg *-- IBridge
    IBridge <.. EthBridge
    NonceSetter <.. EthBridge
    EvmContractBridge <.. EthBridge
    CustomConfig *-- EthBridge
    NonceSetterBase *-- EthBridge
    EthBridge *-- ReefBridge
    EvmContractBridge <.. ReefBridge

```
## Sequence Diagram 

```mermaid
sequenceDiagram

    participant R as Router 
    participant J as JS Scirpt
    participant W as Reef WebSocket(GraphQL)
    participant H as Reef Rpc
    
    R ->> W: connect ws
    loop 
        W->>W: Send and Receive
        Note over W: QueryTx QueryEventLogs QueryEvmAddress QueryReefAddress
    end

    R ->>+ J: exec js
    Note over J: GetSignInfo BuildSigningMessage GetTxHash SendSignedTx SignMessageWithPrivate 
    J ->>+ H: request
    H ->>- J: response
    J ->>- R: result

    R ->>+ H: request
     Note right of R: others
    H ->>- R: response
```

# Tools
- GetReefChainID
  ```
  go run tokens/reef/tools/getStubChainID/main.go -h
  ```
- GetReefAddress
   ```
  go run tokens/reef/tools/getReefAddress/main.go -h
  ```
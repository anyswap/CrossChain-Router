# Deployment

### 1. chainId
mainnet: 1645027868239
testnet(preprod): 1645027868240

### 2. url
mainnet: https://graphql-api.mainnet.dandelion.link/
testnet: https://graphql-api.testnet.dandelion.link/

### 3. register blockfrost
https://blockfrost.io/  to get api key

### 4. route config key points
``` 
[Extra.Customs.1645027868240]
APIKey = "api key" # blockfrost APIKEY
AssetPolicyKey = "policy seed" # Policykey seed
AppendName = "true"  #config whether the policy is the same for all asset, policy is same if set `false`
UseAPI = "false"  #config whether use the blockfrost api to query tx or tip .etc
TxTimeout = "600"  #config how many block solt passed when tx timeout, default is 600
ReswapMaxAmountRate = "1" #config What thousandths of BigAmount will not auto reswap when swap amount above, `1` means 1‰
```

### 5. genMpcAddr
go run tokens/cardano/tools/pubKeyToAddr/main.go -h

### 6. get some ADA
Testnet: https://docs.cardano.org/cardano-testnet/tools/faucet

### 7. mint token
go run tokens/cardano/tools/mintToken/main.go -h
   
### 8. set config contract
	7.1 routerAddress is mpc address
	7.2 tokenAddress is policyId.assetName if token not ADA
	    tokenAddress is lovelace if token is ADA

### 9. send swapout tx
go run tokens/cardano/tools/sendTransaction/main.go -h

### 10.tools

- go run tokens/cardano/tools/mintToken/main.go -h
- go run tokens/cardano/tools/pubKeyToAddr/main.go -h
- go run tokens/cardano/tools/newWallet/main.go -h
- go run tokens/cardano/tools/newToken/main.go -h
- go run tokens/cardano/tools/queryUtxos/main.go -h
- go run tokens/cardano/tools/sendTransaction/main.go -h

# DevOps
#### run cardano node

```text
https://developers.cardano.org/docs/get-started/installing-cardano-node/#overview
```

#### Graphql

```text
https://github.com/input-output-hk/cardano-graphql
```

#### minting native tokenclear

```text
https://docs.cardano.org/native-tokens/getting-started
```

#### scaner
tokens/cardano/tools/scan/main.go
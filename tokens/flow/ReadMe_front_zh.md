## flow
合约仓库： https://github.com/anyswap/flow-contract 

## sdk
https://developers.onflow.org/tools/fcl-js

## api
https://developers.onflow.org/http-api/

## method
>1)tokenInfo 
```shell
decimal: 6
name/symbol: contractAddr
```
>2)underlying
```shell
code: https://github.com/anyswap/flow-contract/blob/main/contracts/multichain/AnyExampleToken.cdc
method:  pub fun underlying(): String?
```
>3)native balance
```shell
code: https://github.com/anyswap/flow-contract/blob/main/contracts/core/FungibleToken.cdc
method:  pub fun deposit(from: @Vault)
```
>4)token balance
```shell
code: https://github.com/anyswap/flow-contract/blob/main/contracts/core/FungibleToken.cdc
method: pub fun deposit(from: @Vault)
```
>5)totalSupply
```shell
code: https://github.com/anyswap/flow-contract/blob/main/contracts/core/FungibleToken.cdc
method: pub var totalSupply: UFix64
```
>6)create vault
```shell
code: https://github.com/near/near-sdk-rs/blob/master/near-contract-standards/src/fungible_token/storage_impl.rs
method: pub fun createEmptyVault(): @Vault
```


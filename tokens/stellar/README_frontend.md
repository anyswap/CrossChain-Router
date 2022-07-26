# stellar frontend 

## rpc

https://horizon.stellar.org/ for interacting with the public network
https://horizon-testnet.stellar.org/ for interacting with the testnet

## query token info

see https://stellar.github.io/js-stellar-sdk/AssetsCallBuilder.html

```
example
https://horizon-testnet.stellar.org/assets?asset_code=0&asset_issuer=GDQ4TZ2JQ66S4GKCINHRBYBDO5D4U7FCNALFTCY45O5RUWLPYBC3BJ2N
```

## query native balance

see https://stellar.github.io/js-stellar-sdk/AccountCallBuilder.html
refer the props `balances`
```
example
https://horizon-testnet.stellar.org/accounts/GA7EDFQK7ZOSVEVNCGCMLJVC5SDT5PYFYEKUI5IFNHJUA5PQ6OJMP5GC
```

## token balanceOf 

the same as `query native balance`

## query token total supply

see https://stellar.github.io/js-stellar-sdk/AssetsCallBuilder.html
refer the props `balances.authorized`
```
example
https://horizon-testnet.stellar.org/assets?asset_code=Ooxx&asset_issuer=GCHH22AXHDDXET47Q3YKSANZ74GAAWEIMECQ3ZNFVM3SJY2LGYYPFLUH
```

## build tx memo

see https://github.com/anyswap/CrossChain-Router/blob/feature/stellar/tokens/stellar/README.md#router-mechanism




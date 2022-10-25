1) deploy contract

```text
priceOracle: https://github.com/anyswap/multichain-smart-contracts/blob/dev/contracts/router/PriceFeedRouter.sol
routerConfig: https://github.com/anyswap/router-config/blob/main/contracts/RouterConfigV2.sol
```

2) update router code config

```text
SwapType = "gasswap"

# OnChain config
[OnChain]
APIAddress = ["https://data-seed-prebsc-1-s1.binance.org:8545/"]
Contract = "0x3158a1898571DCA0679a3881A5f6B31fdC9A97EA"

# OnChain config
[PriceOracle]
APIAddress = ["https://data-seed-prebsc-1-s1.binance.org:8545/"]
Contract = "0x758CDFaC7F0f88A27B3a6a18801553eE8D9e4c24"

```

3) set router contract config

```text
3.1 addChainID
3.2 addTokenID
3.3 setChainConfig(routerContract is mpc address)
3.4 setMPCPubkey
```

4) set price oracle config

```text
4.1 initCurrencyInfo
```

5) price feed code

```text
https://github.com/anyswap/priceFeed
```

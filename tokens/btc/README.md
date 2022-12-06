# btc router specials

## btc token config ContractAddress item

`ContractAddress` is `BTC` or format of `Currency`

`btc` stands for native

## btc chain and token config RouterContract item

`RouterContract` is the `mpc` address

## btc public key to btc address

```shell
go run tokens/btc/tools/pubKeyToAddress/main.go -pubKey 0409373c48ead4253c79f0f2063139170c9a3f47d3aa9dc1fb99a2f6edff0703ff55742d2475ea2703c2b8d479221c6df53bf50c9832c94501597a65c40858b4f6
# output
address: mwjkhU9ZdoShS7wPKgj7dE8DnKgdRypoYh
```

## router mechanism

1. Swapout from btc to other chain

user send asset to `mpc` address with memo of the following format

```
bindAddress:toChainID

example:

0x1111111111111111111111111111111111111111:4
```

to specify route asset to which address (`bindAddress`)
and to which destination blockchain (`toChainID`)


2. Swapin from other chain to btc

```solidity
function anySwapOut(address token, string memory to, uint amount, uint toChainID)
```

`to` is the destination on btc, it can be an btc address


## btc tools

use `-h` option to get help info for each tool

example:

```shell
go run tokens/btc/tools/pubKeyToAddress/main.go -h
```

```text
pubKeyToAddress
	convert ec public key to btc address

wifToEcPrivKey
	convert wif to ec privKey
```

config example:
    tokenConfig: [
        {
            chainID: "1000004346948",
            tokenAddr: "btc",
            decimals: 8,
            version: 6,
            routerContract: "mwjkhU9ZdoShS7wPKgj7dE8DnKgdRypoYh",
            extra: ""
        }
    ]
	chainConfigs: [
        {
            chainID: 1000004346948,
            blockChain: "btc",
            routerContract: "mwjkhU9ZdoShS7wPKgj7dE8DnKgdRypoYh",
            confirmations: "1",
            initialHeight: "5",
            extra: ""
        },
        {
            chainID: 4,
            blockChain: "rinkeby",
            routerContract: "0x585DEa293B5c9dfe3E44976FD1A724e11CD7Cc21",
            confirmations: "15",
            initialHeight: "10781362",
            extra: ""
        },
    ]

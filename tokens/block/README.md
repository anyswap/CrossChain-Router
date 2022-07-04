# block router specials

## block token config ContractAddress item

`ContractAddress` is `BLOCK` or format of `Currency`

`block` stands for native

## block chain and token config RouterContract item

`RouterContract` is the `mpc` address

## block public key to block address

```shell
go run tokens/block/tools/pubKeyToAddress/main.go -pubKey 0409373c48ead4253c79f0f2063139170c9a3f47d3aa9dc1fb99a2f6edff0703ff55742d2475ea2703c2b8d479221c6df53bf50c9832c94501597a65c40858b4f6
# output
address: mwjkhU9ZdoShS7wPKgj7dE8DnKgdRypoYh
```

## router mechanism

1. Swapout from block to other chain

user send asset to `mpc` address with memo of the following format

```
bindAddress:toChainID

example:

0x1111111111111111111111111111111111111111:4
```

to specify route asset to which address (`bindAddress`)
and to which destination blockchain (`toChainID`)


2. Swapin from other chain to block

```solidity
function anySwapOut(address token, string memory to, uint amount, uint toChainID)
```

`to` is the destination on block, it can be an block address


## block tools

use `-h` option to get help info for each tool

example:

```shell
go run tokens/block/tools/pubKeyToAddress/main.go -h
```

```text
pubKeyToAddress
	convert ec public key to block address

wifToEcPrivKey
	convert wif to ec privKey
```

config example:
    tokenConfig: [
        {
            chainID: "1000004346948",
            tokenAddr: "block",
            decimals: 8,
            version: 6,
            routerContract: "mwjkhU9ZdoShS7wPKgj7dE8DnKgdRypoYh",
            extra: ""
        }
    ]
	chainConfigs: [
        {
            chainID: 1000004346948,
            blockChain: "block",
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

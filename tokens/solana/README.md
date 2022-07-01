# solana router specials

## solana token config

Native token `SOL` : `contract address` is `native`

Token not issued by multichain : `contract version` is `0`

AnyToken issued by multichain :  `contract version` > `0`

## solana public key to solana address

```shell
go run tokens/solana/tools/publicKeyToAddress/main.go -p 0xedbe0d03d8022012a03d5535e8677681dbbd9bbd130a3593388a61454129f5c294
# output
address: DnsySaKza7ggR6RoviWNWb6WGLg6aKtmYo9dbeuhjQoV
```

## router mechanism

1. Swapout from solana to other chain

user send asset to `mpc` address with memo of the following format

to specify route asset to which address (`bindAddress`)
and to which destination blockchain (`toChainID`)


1. Swapin from other chain to solana

```solidity
function anySwapOut(address token, string memory to, uint amount, uint toChainID)
```

`to` is the destination on solana, it can be an solana address, or `solana_address:destinationTag` for some address that require destination tag.


## solana tools

use `-h` option to get help info for each tool

example:

```shell
go run tokens/solana/tools/publicKeyToAddress/main.go -h
```

```text
publicKeyToAddress
	convert solana public key to solana address

sendPaymentTx
	send mpc signed Payment tx

sendAccountSetTx
	send mpc signed AccountSet tx

sendTrustSetTx
	send mpc signed TrustSet tx
```

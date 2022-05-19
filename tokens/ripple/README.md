# ripple router specials

## ripple token config ContractAddress item

`ContractAddress` is `XRP` or format of `Currency/Issuser`

`XRP` stands for native

## ripple chain and token config RouterContract item

`RouterContract` is the `mpc` address

## ripple public key to ripple address

```shell
go run tokens/ripple/tools/publicKeyToAddress/main.go 04b4904f8a2ea01891678fec45c63fb1f221666e7d19cfeeb28f08a6d99cac91cbc12731f4c144aef501e34a6eaa0b5418ed5d138b192964bc5ccf4cde67246ca3
# output
address: rDsvn6aJG4YMQdHnuJtP9NLrFp18JYTJUf
```

## router mechanism

1. Swapout from ripple to other chain

user send asset to `mpc` address with memo of the following format

```
bindAddress:toChainID

example:

0x1111111111111111111111111111111111111111:4
```

to specify route asset to which address (`bindAddress`)
and to which destination blockchain (`toChainID`)


2. Swapin from other chain to ripple

```solidity
function anySwapOut(address token, string memory to, uint amount, uint toChainID)
```

`to` is the destination on ripple, it can be an ripple address, or `ripple_address:destinationTag` for some address that require destination tag.


## ripple tools

use `-h` option to get help info for each tool

example:

```shell
go run tokens/ripple/tools/publicKeyToAddress/main.go -h
```

for transaction flags, please refer `tokens/ripple/rubblelabs/ripple/data/flags.go`

```text
publicKeyToAddress
	convert ripple public key to ripple address

sendPaymentTx
	send mpc signed Payment tx

sendAccountSetTx
	send mpc signed AccountSet tx

sendTrustSetTx
	send mpc signed TrustSet tx
```

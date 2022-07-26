# stellar router specials

## stellar token config ContractAddress item

`ContractAddress` is `native`(XLM) or format of `Code/Issuser`

`XLM` stands for native

## stellar chain and token config RouterContract item

`RouterContract` is the `mpc` address

## stellar public key to stellar address

```shell
go run ./tokens/stellar/tools/publicKeyToAddress/main.go -p 0xED146f71db711bc259176f9bcba1756308d2a7af0f1c0b90deece65997a84c8f56
# output
address: GAKG64O3OEN4EWIXN6N4XILVMMENFJ5PB4OAXEG65TTFTF5IJSHVMBIC
```

## router mechanism

user send asset to `mpc` address with memo of the following format

```
len(hexBytesArray(bindAddress)) hexBytesArray(bindAddress) zeroPadding(hexBytesArray(toChainID))

example:

bindAddress: 0xC5107334A3Ae117E3DaD3570b419618C905Aa5eC  toChainID:5777

[20 197 16 115 52 163 174 17 126 61 173 53 112 180 25 97 140 144 90 165 236 0 0 0 0 0 0 0 0 0 87 119]

output:
14c5107334a3ae117e3dad3570b419618c905aa5ec0000000000000000001691
```

to specify route asset to which address (`bindAddress`)
and to which destination blockchain (`toChainID`)

## stellar tools

use `-h` option to get help info for each tool

example:

```shell
go run tokens/stellar/tools/publicKeyToAddress/main.go -h
```

```text
addressToPublickey
	convert stellar address to stellar public key(ed25519)
publicKeyToAddress
	convert stellar public key to stellar address
buildSwapMemo
	format swapout memo by toAddress and toChainID
sendPaymentTx
	send mpc signed Payment tx
sendTrustLineTx
	send mpc signed TrustLine tx
getStubChainID
	get stellar chainID
```

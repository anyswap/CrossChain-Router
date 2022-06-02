# stellar router specials

## stellar token config ContractAddress item

`ContractAddress` is `native`(XLM) or format of `Code/Issuser`

`XLM` stands for native

## stellar chain and token config RouterContract item

`RouterContract` is the `mpc` address

## stellar public key to stellar address

```shell
go run tokens/stellar/tools/publicKeyToAddress/main.go 3e41960afe5d2a92ad1184c5a6a2ec873ebf05c11544750569d34075f0f392c7
# output
address: GA7EDFQK7ZOSVEVNCGCMLJVC5SDT5PYFYEKUI5IFNHJUA5PQ6OJMP5GC
```

## router mechanism

user send asset to `mpc` address with memo of the following format

```
len(hexBytesArray(bindAddress)) hexBytesArray(bindAddress) zeroPadding(hexBytesArray(toChainID))

example:

bindAddress: 0xC5107334A3Ae117E3DaD3570b419618C905Aa5eC  toChainID:5777

[20 197 16 115 52 163 174 17 126 61 173 53 112 180 25 97 140 144 90 165 236 0 0 0 0 0 0 0 0 0 87 119]
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
publicKeyToAddress
	convert stellar public key to stellar address

sendPaymentTx
	send mpc signed Payment tx

sendAccountSetTx
	send mpc signed AccountSet tx

sendTrustSetTx
	send mpc signed TrustSet tx
```

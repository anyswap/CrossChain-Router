# aptos router

## github
https://github.com/anyswap/CrossChain-Router/tree/feature/aptos

## router contract
https://github.com/anyswap/router-aptos-contract/tree/use-table


## router mechanism

atpos contract called `Move` 

https://diem.github.io/move/modules-and-scripts.html



1. Swapout from aptos to other chain
   
	call router contract function `swapout`
	```
	 public entry fun swapout<CoinType>(account: &signer, amount: u64, _receiver: string::String, _toChainID: u64) acquires RouterBurnCap, TokenInfo, SwapOutEventHolder {  
	```


2. Swapin from other chain to aptos

	map call router contract function `swapin`
	```
	public entry fun swapin<CoinType, PoolCoin>(admin: &signer, receiver: address, amount: u64, _fromEvent: string::String, _fromChainID: u64) acquires RouterMintCap,TokenInfo,SwapInEventHolder
	```

## aptos tools

use `-h` option to get help info for each tool

```shell
# get chainid 
go run tokens/aptos/tools/getStubChainID/main.go -h
# convert publickey HexString to aptos address
go run tokens/aptos/tools/publicKeyToAddress/main.go -h
# deployed modules by mpc or privatekey signer
go run tokens/aptos/tools/deployModule/main.go -h
# register PoolCoin in pool contract to init LP
go run tokens/aptos/tools/registerPoolCoin/main.go -h
# config coin type in router contract
go run tokens/aptos/tools/configCoin/main.go -h
# copy mint MintCapability to router
go run tokens/aptos/tools/setPoolcoinCap/main.go -h
# issue token 
go run tokens/aptos/tools/issueToken/main.go -h

```

```shell
Example:
go run tokens/aptos/tools/publicKeyToAddress/main.go -p 0xedbe0d03d8022012a03d5535e8677681dbbd9bbd130a3593388a61454129f5c294
# output
address: 0x08129c35bfef3fd283da15a2f50c261a0ae3f1d1f500fa5829c3a40cf4dda15d
```

## about aptos 
API：https://fullnode.devnet.aptoslabs.com/v1/spec#/
explore: https://explorer.devnet.aptos.dev/

### notice
aptos use ED25591 algorithm to create account

> devnet  
http:  http://fullnode.devnet.aptoslabs.com
chain_id:  1000004280406

> testnet  
http:  https://api.testnet.aptos.com
chain_id:  1000004280405

> mainnet  
http: TBA


### js sdk
web3: https://aptos.dev/sdks/typescript-sdk






# solana router deploy instructions

### 1. setup CrossChain-Router solana config
```
# auto resign tx if nonce passed, key: chainid  value: 1:on 0:off
[Server.AutoResignTxIfFailed]
245022926 = 1
245022940 = 1
245022934 = 1

[Gateways]
245022934 = ["https://api.mainnet-beta.solana.com"]  #mainnet
245022940 = ["https://api.testnet.solana.com"] #testnet
245022926 = ["https://api.devnet.solana.com"] #devnet
```

### 2. solana token config
default token decimal is `9`

Native token `SOL` : `contract address` is `native`
Token not issued by multichain : `contract version` is `0`
AnyToken issued by multichain :  `contract version` > `0`

### 3. create payer account
support delegetor pay is supported in solana,so strongly suggest to create sonala payer
```
go run tokens/solana/tools/genAccount/main.go
# output
PriKey bytes:
PriKey(base58):
Address(base58):
```
 
### 4. router contract deploy
See
https://github.com/anyswap/router-solana-contract#install-and-config

### 5. issue new token
```
go run tokens/solana/tools/issueToken/main.go -h
```

### 6. swapout 
see
https://github.com/anyswap/router-solana-contract#Tools

```
node app/client.js -h             
Usage: client [options] [command]

Options:
  -V, --version               output the version number
  -h, --help                  display help for command

Commands:
  swapout-native [options]
  swapout-burn [options]
  swapout-transfer [options]
  mint [options]
  help [command]              display help for command
```

```
node app/client.js swapout-native -h
Usage: client swapout-native [options]

Options:
  -t --to <to>            to address
  -a --amount <amount>    to address
  -c --chainid <chainid>  to chainid
  -o --owner <owner>      router owner account
  -h, --help              display help for command
Examples:

ANCHOR_PROVIDER_URL=http://localhost:8899 ANCHOR_WALLET=/Users/potti/.config/solana/id.json node app/client.js swapout-native -a 1000000000 -c 5777 -t 0xC5107334A3Ae117E3DaD3570b419618C905Aa5eC -o GdkWDfbwCe9KE8JB9JWLvYnXpgcYiReB7DTfeqvZaxgW
```









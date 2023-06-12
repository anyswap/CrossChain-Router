## Get Block Txns
Get latest block
```bash
go run ./tokens/starknet/tools/getBlock/main.go --network https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f
```

Get block by number
```bash
go run ./tokens/starknet/tools/getBlock/main.go --network https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f --number 123
```

Get block by hash
```bash
go run ./tokens/starknet/tools/getBlock/main.go --network https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f --hash 0x075e00250d4343326f322e370df4c9c73c7be105ad9f532eeb97891a34d9e4a5
```

## params

```text
testnet: https://api.lb-0.h.chrysalis-devnet.iota.cafe
mainnet: https://chrysalis-nodes.iota.cafe
chainId: 
    mainnet: 1001229935681
    testnet: 1001229935682
faucet: https://faucet.chrysalis-devnet.iota.cafe/
api: https://editor.swagger.io/?url=https://raw.githubusercontent.com/iotaledger/tips/main/tips/TIP-0013/rest-api.yaml
explorer: https://explorer.iota.org/devnet
```

## tools

```text
1) getStubChainID
go run ./tokens/iota/tools/getStubChainID/main.go -p testnet

testnet: 1001229935682
```

```text
2) publicKeyToAddress
go run ./tokens/iota/tools/publicKeyToAddress/main.go -p https://api.lb-0.h.chrysalis-devnet.iota.cafe -publicKey eaf07201ddba01c9537fea8cbdcf3bda7726b0155808f2442c62d48797dc55d4

edAddr: 9fb648524b9747608791dbd76bacbebc2f7ac0e3ace10e896739a0a44190102f
iotaAddr: atoi1qz0mvjzjfwt5wcy8j8daw6avh67z77kquwkwzr5fvuu6pfzpjqgz7cptqex
```

```text
3) sendTransaction
go run ./tokens/iota/tools/sendTransaction/main.go -n https://api.lb-0.h.chrysalis-devnet.iota.cafe -publicKey ae67243f1d58f57188335adc29a98e856e46c0cc38bd63558d6636fcd0eccc03 -privKey 4023b0cc68bff18b8ae595b3a1f315f925d187b083ffcdee5051a70086bbec0cae67243f1d58f57188335adc29a98e856e46c0cc38bd63558d6636fcd0eccc03 -to atoi1qz0mvjzjfwt5wcy8j8daw6avh67z77kquwkwzr5fvuu6pfzpjqgz7cptqex -index swapOut -data 0x1739648E7C1B23D6Da74177292B36aF7f286f643:4 -value 1000000
```

```text
4) keygen
go run ./tokens/iota/tools/keygen/main.go
```

## deploy step

```text
1) get mpc publicKey(ed25519)
2) get mpc iota address
go run ./tokens/iota/tools/publicKeyToAddress/main.go -p https://api.lb-0.h.chrysalis-devnet.iota.cafe -publicKey eaf07201ddba01c9537fea8cbdcf3bda7726b0155808f2442c62d48797dc55d4

edAddr: 9fb648524b9747608791dbd76bacbebc2f7ac0e3ace10e896739a0a44190102f
iotaAddr: atoi1qz0mvjzjfwt5wcy8j8daw6avh67z77kquwkwzr5fvuu6pfzpjqgz7cptqex
3) set config 
chainConfig: routerContract is mpc address(edAddr)
tokenConfig: routerContract is mpc address(edAddr) tokenAddr is "iota" decimal:6
mpcPublicKey: addr is edAddr
4) swapin
iotaAddr:chainID
5) swapOut
go run ./tokens/iota/tools/sendTransaction/main.go -n https://api.lb-0.h.chrysalis-devnet.iota.cafe -publicKey ae67243f1d58f57188335adc29a98e856e46c0cc38bd63558d6636fcd0eccc03 -privKey 4023b0cc68bff18b8ae595b3a1f315f925d187b083ffcdee5051a70086bbec0cae67243f1d58f57188335adc29a98e856e46c0cc38bd63558d6636fcd0eccc03 -to [mpcIotaAddr] -index swapOut -data 0x1739648E7C1B23D6Da74177292B36aF7f286f643:4 -value 1000000
```

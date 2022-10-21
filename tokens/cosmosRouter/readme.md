## cosmosHub

```text
1) chainId
mainnet: 1293698254146
testnet: 1293698254147

2) apiUrl
mainnet: https://cosmos-mainnet-rpc.allthatnode.com:1317
testnet: https://cosmos-testnet-rpc.allthatnode.com:1317

3) explorer:
https://atomscan.com/

4) faucet:
https://www.allthatnode.com/faucet/cosmos.dsrv
```

## tools

```text
1) getChainId
go run ./tokens/cosmosSDK/tools/getStubChainID/main.go -n COSMOSHUB -p testnet

testnet: 1293698254147

2) publicKeyToAddress
go run ./tokens/cosmosSDK/tools/publicKeyToAddress/main.go -p 0x0468438a94627b0de2b6a7c9af99136ef7e607f7944b749c3534bb27a89e742d583b1c8b3aecfae45dea2ac58730aa6ba654c73c435d44755e5cd1500c8f4d036b -prefix cosmos

addr:cosmos10yyn2er9k5cs9qn55l7t23yxxk7egecpw9lw90
```

## deploy step

```text
1) chainConfig
routerContract: mpc address
extra: addressPrifix cosmosHub: cosmos

2) tokenConfig
tokenAddress: uatom 
decimals: 0

3) example
https://rinkeby.etherscan.io/address/0x4342F2b5224a43541BE7C8F39B92D7fEaA74d038
```

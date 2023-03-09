# Reef router deploy instructions

## Network
Make sure the rpc endpoint could be connected
| Env | Rpc Endpoint | Graphql |
| ---- | ----  | ----  |
| Mainnet | https://rpc.reefscan.com | wss://reefscan.com/graphql |
| Testnet | https://rpc-testnet.reefscan.com | wss://testnet.reefscan.com/graphql |


##  Setup Reef contract 
### a. run the lastest mpc node 
- use mpc `support-sr25519` to generate a new mpc public key base on sr25519
https://github.com/anyswap/FastMulThreshold-DSA/tree/support-sr25519

```
./gsmpc-test-2-2.sh $(pwd) 5871 "" SR25519
```

### b. create mpc account
- get reef address (mpc SS58 Address)
```
go run tokens/reef/tools/getReefAddress/main.go -h
```
- get some native token `REEF` to mpc address, airdrop for testnet 
https://app.element.io/#/room/#reef:matrix.org

- git clone `https://github.com/anyswap/Router-Demo-JS/tree/reef` and setup js env 
```
npm install -g yarn
yarn i
```
  
- random a evm private key(mpc EVM address) to bind reef address
  
- use `bindEvmaddr` tools to bind evm address
  ```
  go run tokens/reef/tools/bindEvmaddr/main.go -h
  ```

### c. contract deploy
- perpare a deploying account which contains `REEF`
- use https://github.com/reef-defi/hardhat-reef to deploy
- the mpc address is gotten above(mpc EVM address)

## Setup CrossChain-Router 
### a.setup reef config

```
[Gateways]
1001380271430 = ["https://rpc.reefscan.com/"]
1001380271431 = ["https://rpc-testnet.reefscan.com/"]

[Extra.Customs.1001380271430]
ws = "wss://reefscan.com/graphql,wss://reefscan.com/graphql"
jspath = "/Router-Demo-JS/reef"

[Extra.Customs.1001380271431]
ws = "wss://testnet.reefscan.com/graphql,wss://testnet.reefscan.com/graphql"
jspath = "/Router-Demo-JS/reef"
```
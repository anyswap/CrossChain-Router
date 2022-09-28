# aptos router deploy instructions

## 1. Setup aptos contract 
### a. create mpc account
aptos contract is a part of `Account Modules`, so only mpc have to deploy all the contract
get mpc public and run publicKeyToAddress tools to get aptos address
```
go run tokens/aptos/tools/publicKeyToAddress/main.go -h
```
get some native token `APT` to mpc address
### b. contract deploy
deploy `Pool` and `Router` 
1. git clone https://github.com/anyswap/router-aptos-contract/tree/use-table
2. copy mpc address ,  replace `Multichain`'s address at `router-aptos-contract/blob/use-table/router/Move.toml`
3. cd  router-aptos-contract and run `aptos move test --package-dir router` to test code
4. run `aptos move compile --package-dir router` to build code 
5. use the deployModule to deploy , modules and path like `-module Pool,Router -path ../aptos-contract/router/build/multichain` 
```
go run tokens/aptos/tools/deployModule/main.go -h
```

### c. issue new token
1. add new struct name `AnyMyCoin` as `anyUSDT` in anycoin/sources/PoolCoin.move
2. use the deployModule to deploy , modules is the path like `-module ../aptos-contract/anycoin/build/anycoin/bytecode_modules/PoolCoin.mv` 
```
go run tokens/aptos/tools/deployModule/main.go -h
```
3. use `issueCoin` to issue a coin 

### d. register token to self
```
go run tokens/aptos/tools/registerCoin/main.go -h
```

### e. mint token to address
```
go run tokens/aptos/tools/mintCoin/main.go -h
```

### f. register lp in pool
call registerPoolCoin to new a token and register a LP in pool
1. deploy anytoken like `PoolCoin` (notice: if deploy multi anytoken, need to create new contract, eg. copy `PoolCoin` and rename to `anyXXCoin`, check the struct name `AnyMyCoin` to `anyXXCoin`)
2. deploy the module
3. use `registerPoolCoin`  tools 
```
go run tokens/aptos/tools/registerPoolCoin/main.go -h
```

### g. set coin type
anyCoin set 0
underlying Coin set 1
```
go run tokens/aptos/tools/configCoin/main.go -h
```

### g. set router status
router status `[1]:open [0]:close`
```
go run tokens/aptos/tools/setStatus/main.go -h
```


## 2. Setup CrossChain-Router Enviroment
### a. setup aptos config
```
[Gateways]
1000004280406 = ["http://fullnode.devnet.aptoslabs.com"] #devnet
1000004280405 = ["https://testnet.aptoslabs.com"]
```

### b. config contract aptos token config
native token decimal is `8`

set extra `underlying recource` if token is anyCoin
```
instance.setTokenConfig("AnyMyCoin", "1000004280406", "0xc441fa1354b4544457df58b7bfdf53fae75e0d6f61ded55b72ae058d2d407c9d::PoolCoin::AnyMyCoin", 6, 6, "0xc441fa1354b4544457df58b7bfdf53fae75e0d6f61ded55b72ae058d2d407c9d", "0x27b1c07abb2146204ba281464ace56075c7d1338a8df0fbe44245674b6fa1309::TestCoin::MyCoin")
```

### c. swapout 
```
go run tokens/aptos/tools/swapout/main.go -h
```








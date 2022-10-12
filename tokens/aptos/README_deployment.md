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
4. run `aptos move compile --save-metadata --package-dir router` to build code 
5. use the deployModule to deploy , modules and path like `-module Pool,Router -path ../aptos-contract/router/build/multichain` 
```
go run tokens/aptos/tools/deployModule/main.go -h
```

### c. deploy anyCoin
1. add new struct name `AnyMyCoin` as `anyUSDT` in anycoin/sources/PoolCoin.move
2. run `aptos move compile --save-metadata --package-dir anycoin` to build code
3. use the deployModule to deploy , modules is the path like `-module PoolCoin  -path ../aptos-contract/anycoin/build/anycoin` 
```
go run tokens/aptos/tools/deployModule/main.go -h
```

### d. deploy and issue underlyingCoin
1. copy `aptos-contract/anycoin/sources/wETH.move` to `aptos-contract/anycoin/sources/{coinName}.move`
2. change the coin name in line 2,35,36  with your coinName
```
module TEST::wETH {

string::utf8(b"wETH"),
string::utf8(b"wETH"),

```
3. run `aptos move compile --save-metadata --package-dir anycoin` to build code
4. use the deployModule to deploy , modules is the path like `-module {coinName}  -path ../aptos-contract/anycoin/build/anycoin` 

### e. register coin to user so that user can hold coin
```
go run tokens/aptos/tools/registerCoin/main.go -h
```
you should register anytoken and underlying both

### f. mint coin to user for test
```
go run tokens/aptos/tools/mintCoin/main.go -h
```

### g. register lp in pool
call registerPoolCoin to new a poolCoin and register a LP in pool
1. deploy anytoken like `PoolCoin` (notice: if deploy multi anytoken, need to create new contract, eg. copy `PoolCoin` and rename to `anyXXCoin`, check the struct name `AnyMyCoin` to `anyXXCoin`)
2. deploy the module
3. use `registerPoolCoin`  tools 
```
go run tokens/aptos/tools/registerPoolCoin/main.go -h
```

### h. set coin type 
1. anyCoin set 0
2. underlying Coin set 1
```
go run tokens/aptos/tools/configCoin/main.go -h
```

### i. set router status (admin)
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








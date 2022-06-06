## router
路由仓库： https://github.com/anyswap/CrossChain-Router/tree/feature/flow

## flow
合约仓库： https://github.com/anyswap/flow-contract 

常用api文档：https://docs.onflow.org/http-api/

> testnet  
rpc:  https://rest-testnet.onflow.org/v1/  
chain_id:  1001179406168
import FungibleToken from 0x9a0766d93b6608b7
import FlowToken from 0x7e60df042a9c0868
import NonFungibleToken from 0x631e88ae7f1d7c20

> mainnet  
rpc:  https://rest-mainnet.onflow.org/v1/  
chain_id:  1001179406167
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import NonFungibleToken from 0x1d7e57aa55817448

## router部署文档 
https://github.com/anyswap/CrossChain-Router/tree/feature/near#readme
## mpc部署文档 
https://github.com/anyswap/FastMulThreshold-DSA/wiki/keygen-and-sign-workflow

## 合约参考(已验证)
> rinkeby_testnet  
config: 0x5fC5b2500f3820839D83B8CF9254Ff50F2ffcC2C  
router: 0x585DEa293B5c9dfe3E44976FD1A724e11CD7Cc21   
anyToken: 0x374c005D004fFf1431Ed4E3A7943d370298E7732 

> flow_testnet  
router: A.address.Router  
anytoken: A.address.contractName  
underlying: A.address.contractName

***
特别强调  
>1) mpc公钥和flow公钥以及account.key的关系  
mpc申请ec公钥后，去掉首字节04极为flow公钥，替换为0x则为account.key

即flow公钥开头加04为mpc公钥，加0x为account.key
假设mpc公钥为：041234
flow公钥即为：1234
account.key即为：0x1234
***

## flow合约部署
>1) 部署anytoken合约规范  
```shell
go run ./tokens/flow/tools/deployContract/AnyToken/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx 
```
>2) 部署anytoken合约实现
```shell
go run ./tokens/flow/tools/deployContract/AnyExampleToken/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx 
```
>3) 部署router合约实现
```shell
go run ./tokens/flow/tools/deployContract/Router/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx 
```
4) mpc init
```shell
go run ./tokens/flow/tools/initMpc/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx 
```
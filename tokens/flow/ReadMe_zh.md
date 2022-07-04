## router
路由仓库： https://github.com/anyswap/CrossChain-Router/tree/feature/flow

## flow
合约仓库： https://github.com/anyswap/flow-contract 

常用api文档：https://docs.onflow.org/http-api/

> testnet  
rpc:  https://rest-testnet.onflow.org/v1/  
grpc: access.devnet.nodes.onflow.org:9000
chain_id:  1001179406168
scan: https://testnet.flowscan.org/
import FungibleToken from 0x9a0766d93b6608b7
import FlowToken from 0x7e60df042a9c0868
import NonFungibleToken from 0x631e88ae7f1d7c20

> mainnet  
rpc:  https://rest-mainnet.onflow.org/v1/  
grpc: access.mainnet.nodes.onflow.org:9000  
chain_id:  1001179406167
scan: https://flowscan.org/
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
router: A.2627a6b6570638c4.Router  
AnyToken: A.2627a6b6570638c4.AnyToken  
ExampleToken: A.2627a6b6570638c4.ExampleToken  
AnyExampleToken: A.2627a6b6570638c4.AnyExampleToken  

## 交易参考
rinkeby_swapOut:  https://rinkeby.etherscan.io/tx/0xfade152f61c24c24cf5083ba6b02ea4c347a146d3b68db857d265fac4564a7bc
flow_swapIn:  https://testnet.flowscan.org/transaction/30abc93d4c2b467230aaf2173245ffee8303408d18400c7354f1455d25be5219
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
>0) 部署example合约（模拟underlying）
```shell
go run ./tokens/flow/tools/deployContract/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx -contract ExampleToken -privKey(option) xxx
```
>1) 部署anytoken合约规范  
```shell
go run ./tokens/flow/tools/deployContract/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx -contract AnyToken -privKey(option) xxx
```
>2) 部署anytoken合约实现
```shell
go run ./tokens/flow/tools/deployContract/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx -contract AnyExampleToken -privKey(option) xxx
```
>3) 部署router合约实现
```shell
go run ./tokens/flow/tools/deployContract/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx -contract Router -privKey(option) xxx
```
4) mpc init 
```shell
go run ./tokens/flow/tools/initMpc/main.go -config xxx.toml -chainID xxx -address xxx -pubKey xxx  -privKey(option) xxx
```

## config 参考
```shell
pubkey:
        {
            addr: "0x2627a6b6570638c4",
            pubkey: "cb146d3d88ed84bebc26be866e18e6e0a690c092ab9d000f673cc0181257845c0e40f343ef781128dfced5445cdb93177947cafc3228619983f16c074924772f"
        }
tokenConfig:
        {
            chainID: "1001179406168",
            tokenAddr: "((UFix64):A.2627a6b6570638c4.AnyExampleToken.Vault)",
            decimals: 8,
            version: 6,
            routerContract: "A.0x2627a6b6570638c4.Router",
            extra: "exampleTokenReceiver,anyExampleTokenReceiver"
        }
chainConfig:
        {
            chainID: 1001179406168,
            blockChain: "flow",
            routerContract: "A.0x2627a6b6570638c4.Router",
            confirmations: "15",
            initialHeight: "5",
            extra: "0x2627a6b6570638c4"
        }
```
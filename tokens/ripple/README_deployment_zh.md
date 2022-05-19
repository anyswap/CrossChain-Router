## router
合约仓库： https://github.com/anyswap/CrossChain-Router/tree/feature/ripple

## ripple
> 特别注意：ripple没有合约的概念，不需要部署合约，采用记账的方式替代token部署
常用api文档：https://xrpl.org/public-servers.html

> devnet  
wss:  wss://s.devnet.rippletest.net/
chain_id:  1000005788242

> testnet  
wss:  wss://s.altnet.rippletest.net:51233  
chain_id:  1000005788241

> mainnet  
wss: wss://s1.ripple.com/  
chain_id: 1000005788240

## router部署文档 
https://github.com/anyswap/CrossChain-Router/tree/feature/near#readme
## mpc部署文档 
https://github.com/anyswap/FastMulThreshold-DSA/wiki/keygen-and-sign-workflow

## 合约参考(已验证)
> bsc_testnet  
config: 0xb89070887e2706cE35642ca430Bb26590B089Ced  
router: 0x6f6a05a62277A9A58F136c0E92e1106B53d9776C  

> 交易参考(bsc->ripple)  
https://testnet.bscscan.com/tx/0x71157069393af79fc559e22003ff8ff8b346af9abfd22db1c33bf6e20b7638a0
https://testnet.xrpl.org/transactions/68FF1258F74709DAA6863C43C9827A13ECBD558D1D8B15CE995BC4F775AA451D
***
特别强调  
>1) mpc公钥和ripple地址的关系  
mpc申请ed公钥后，公钥本身经过地址转换就是ripple的地址，触发一笔交易后即激活  
另外，mpc获取的公钥，通过  https://github.com/anyswap/CrossChain-Router/blob/feature/ripple/tokens/ripple/tools/publicKeyToAddress/main.go  工具可获得ripple address  
示例：  
go run tokens/ripple/tools/publicKeyToAddress/main.go 04b4904f8a2ea01891678fec45c63fb1f221666e7d19cfeeb28f08a6d99cac91cbc12731f4c144aef501e34a6eaa0b5418ed5d138b192964bc5ccf4cde67246ca3  
#output
address: rDsvn6aJG4YMQdHnuJtP9NLrFp18JYTJUf  
>2) ripple转账注意事项  
所有的token接收对象，都必须调用TrustSet方法对接受的token添加信任线授权
>3) config合约配置注意事项  
    3.1 ripple的router_contract都填写为mpc的ripple地址  
    3.2 ripple的token的contractaAddr填写格式为Current/issure  
    3.3 ripple的native(XRP)的contractAddress填写为XRP
>4) ripple调用方法  
    4.1 golang: https://github.com/anyswap/CrossChain-Router/tree/feature/ripple/tokens/ripple/tools  
    // 待测试,测试完更新文档  
    4.2 nodejs: 待上传

## ripple测试步骤
evm部署步骤这里不做赘述

**// 所有的跨出memo写法统一为 bindAddr:toChainId**
>1)ripple原生货币XRP
```shell
1. 跨出
    1.1 ripple链上发起Payment方法，接收对象为mpc地址
    {
        "TransactionType": "Payment",
        "Account": wallet.address,
        "Amount": "xxxx",
        "Destination": mpc,
        "Memos": [
            {
                "Memo": {
                    "MemoData": Buffer.from(bindAddr:toChainId, 'utf8').toString('hex').toUpperCase()
                }
            }
        ],
    })
    1.2 注册交易
2. 跨入
    2.1 源联上发起跨出anyswapout交易，bind字段需要特别注意
    function anySwapOut(address token, string memory to, uint amount, uint toChainID)
    // 如果目标用户有设置tfRequireDestTag，则为bindAddr:tag
    // 如果目标用户没有设置tfRequireDestTag，则为bindAddr
    2.2 注册交易
```

>2)ripple原生token
```shell
1 跨出
    1.1 ripple链上发起TrustSet方法，调用人为mpc，信任对象为currency_code和issure
    {
        "TransactionType": "TrustSet",
        "Account": mpc,
        "LimitAmount": {
            "currency": currency_code,
            "issuer": issure.address,
            "value": "xxxxxx" // Large limit, arbitrarily chosen
        }
    }
    1.2 ripple链上发起Payment方法，接收对象为mpc地址
    {
        "TransactionType": "Payment",
        "Account": wallet.address,
        "Amount": {
            "currency": currency_code,
            "value": "xxxxxxx",
            "issuer": issure.address
        },
        "Destination": mpc,
        "Memos": [
            {
                "Memo": {
                    "MemoData": Buffer.from(bindAddr:toChainId, 'utf8').toString('hex').toUpperCase()
                }
            }
        ],
        "DestinationTag":1, //非必须，接收用户设置了tfRequireDestTag则需要填写
    }
    1.3 注册交易
2 跨入
    2.1 源联上发起跨出anyswapout交易，bind字段需要特别注意
    function anySwapOut(address token, string memory to, uint amount, uint toChainID)
    // 如果目标用户有设置tfRequireDestTag，则为bindAddr:tag
    // 如果目标用户没有设置tfRequireDestTag，则为bindAddr
    2.2 注册交易
``` 

>3)ripple没有token，发行anyToken
```shell
1 发行anyToken
    1.1 ripple链上发起AccountSet方法，调用人为mpc(改方法只需执行一次，后面发行不需要调用)
    {
        "TransactionType": "AccountSet",
        "Account": mpc,
        "TransferRate": 0,
        "TickSize": 5,
        "Domain": "6578616D706C652E636F6D", // "example.com"
        "SetFlag": xrpl.AccountSetAsfFlags.asfDefaultRipple, 
        // 必须配置为asfDefaultRipple，否则发行的token无法自由流转
    }
2 跨出
    // 同原生token跨出流程
3 跨入
    // 同原生token跨入流程
```

## 常见问题
>1)tecPATH_DRY  
```shell
接收地址没有调用TrustSet，无法接收token
```
>2)tecDST_TAG_NEEDED  
```shell
接收对象设置了tfRequireDestTag，转账交易需要填写DestinationTag
```


  

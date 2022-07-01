# router
合约仓库： https://github.com/anyswap/CrossChain-Router/tree/feature/solana

# solana
常用api文档：https://xrpl.org/public-servers.html

> devnet  
wss:  wss://s.devnet.solanatest.net/  
chain_id:  1000005788242

> testnet  
wss:  wss://s.altnet.solanatest.net:51233  
chain_id:  1000005788241

> mainnet  
wss: wss://s1.solana.com/  
chain_id: 1000005788240

## router部署文档 
https://github.com/anyswap/CrossChain-Router/tree/feature/near#readme
## mpc部署文档 
https://github.com/anyswap/FastMulThreshold-DSA/wiki/keygen-and-sign-workflow

## 合约参考(已验证)
> bsc_testnet  
config: 0xb89070887e2706cE35642ca430Bb26590B089Ced  
router: 0x6f6a05a62277A9A58F136c0E92e1106B53d9776C  

> 交易参考(bsc->solana)  
https://testnet.bscscan.com/tx/0x71157069393af79fc559e22003ff8ff8b346af9abfd22db1c33bf6e20b7638a0
https://testnet.xrpl.org/transactions/68FF1258F74709DAA6863C43C9827A13ECBD558D1D8B15CE995BC4F775AA451D
***
特别强调  
>1)mpc公钥和solana地址的关系  
mpc申请ec公钥(65字节16进制编码字符串，非压缩公钥)后，公钥本身经过地址转换就是solana的地址，触发一笔交易后即激活  
另外，mpc获取的公钥，通过  https://github.com/anyswap/CrossChain-Router/blob/feature/solana/tokens/solana/tools/publicKeyToAddress/main.go  工具可获得solana address  
示例：  
go run tokens/solana/tools/publicKeyToAddress/main.go 04b4904f8a2ea01891678fec45c63fb1f221666e7d19cfeeb28f08a6d99cac91cbc12731f4c144aef501e34a6eaa0b5418ed5d138b192964bc5ccf4cde67246ca3  
#output
address: rDsvn6aJG4YMQdHnuJtP9NLrFp18JYTJUf  
>2)solana转账注意事项  
所有的token接收对象，都必须调用TrustSet方法对接受的token添加信任线授权  
>3)config合约配置注意事项  
    3.1 solana的router_contract都填写为mpc的solana地址  
    3.2 solana的token的contractaAddr填写格式为Current/issure  
    3.3 solana的native(XRP)的contractAddress填写为XRP

## solana测试步骤
**evm部署步骤这里不做赘述**

**// 所有的跨出memo写法统一为 bindAddr:toChainId**
>1)solana原生货币XRP
```shell
1. 跨出
    1.1 solana链上发起Payment方法，接收对象为mpc地址
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

>2)solana原生token
```shell
1 跨出
    1.1 solana链上发起TrustSet方法，调用人为mpc，信任对象为currency_code和issure
    {
        "TransactionType": "TrustSet",
        "Account": mpc,
        "LimitAmount": {
            "currency": currency_code,
            "issuer": issure.address,
            "value": "xxxxxx" // Large limit, arbitrarily chosen
        }
    }
    1.2 solana链上发起Payment方法，接收对象为mpc地址
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

>3)solana没有token，发行anyToken
```shell
1 发行anyToken
    1.1 solana链上发起AccountSet方法，调用人为mpc(改方法只需执行一次，后面发行不需要调用)
    {
        "TransactionType": "AccountSet",
        "Account": mpc,
        "TransferRate": 0,
        "TickSize": 5,
        "Domain": "6578616D706C652E636F6D", // "example.com"
        "SetFlag": xrpl.AccountSetAsfFlags.asfDefaultsolana, 
        // 必须配置为asfDefaultsolana，否则发行的token无法自由流转
    }
2 跨出
    // 同原生token跨出流程
3 跨入
    // 同原生token跨入流程
```

## solana调用方法
>1)golang
```shell
脚本路径： https://github.com/anyswap/CrossChain-Router/tree/feature/solana/tokens/solana/tools

1.1 payment Xrp/Token  
    go run ./tokens/solana/tools/sendPaymentTx/main.go -config config.toml -chainID chainid -priKey ecPriKey -pubkey ecPublicKey -destination receiptAddr -amount 100000000  
    //mpc调用的话，去掉priKey参数
    //amount只传数字代表发送xrp，传value/code/issure代表发送token
    //(如100000000/FOO/rUFycYoMtiXcU25CC9v6SdGxDxYvgqxzxS)
1.2 trustSet 
    go run ./tokens/solana/tools/sendTrustSetTx/main.go -config ./build/bin/config-example.toml -chainID chainid -priKey ecPriKey -pubkey ecPublicKey -limitAmount value  
    //mpc调用的话，去掉priKey参数
    //limitAmount传value/code/issure
1.3 AccountSet
    go run ./tokens/solana/tools/sendAccountSetTx/main.go -config ./build/bin/config-example.toml -chainID chainid -priKey ecPriKey -pubkey ecPublicKey -setFlag 0x00000008  
    //mpc调用的话，去掉priKey参数
    //setFlag固定为0x00000008，代表TxDefaultsolana(允许自由流转)
```

## 常见问题
>1)tecPATH_DRY  
```shell
接收地址没有调用TrustSet，或者issure没有设置TxDefaultsolana，无法接收token
```
>2)tecDST_TAG_NEEDED  
```shell
接收对象设置了tfRequireDestTag，转账交易需要填写DestinationTag
```

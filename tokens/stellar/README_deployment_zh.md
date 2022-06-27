# router
合约仓库： https://github.com/anyswap/CrossChain-Router/tree/feature/stellar

# stellar
> 特别注意：stellar没有合约的概念，不需要部署合约，采用记账的方式替代token部署
常用api文档：https://developers.stellar.org/api/

> testnet  
http:  https://horizon-testnet.stellar.org/
chain_id:  1000005786703

> mainnet  
http: https://horizon.stellar.org/  
chain_id:  1000005786702

## router部署文档 
https://github.com/anyswap/CrossChain-Router/tree/feature/stellar#readme
## mpc部署文档 
https://github.com/anyswap/FastMulThreshold-DSA/wiki/keygen-and-sign-workflow


***
特别强调  
>1)mpc公钥和stellar地址的关系  
mpc申请ed公钥(65字节16进制编码字符串，非压缩公钥)后，公钥本身经过地址转换就是stellar的地址，触发一笔交易后即激活  
另外，mpc获取的公钥，通过  https://github.com/anyswap/CrossChain-Router/blob/feature/stellar/tokens/stellar/tools/publicKeyToAddress/main.go  工具可获得stellar address  
示例：  
go run ./tokens/stellar/tools/publicKeyToAddress/main.go -p 0xED146f71db711bc259176f9bcba1756308d2a7af0f1c0b90deece65997a84c8f56
#output
address: GAKG64O3OEN4EWIXN6N4XILVMMENFJ5PB4OAXEG65TTFTF5IJSHVMBIC  
>2)stellar转账注意事项  
所有的token接收对象，都必须设置ChangeTrust方法对接受的token添加信任
>3)config合约配置注意事项, 且stellar链上asset decimal默认为7
    3.1 stellar的router_contract都填写为mpc的stellar地址  
    3.2 stellar的token的contractaAddr填写格式为
    3.3 stellar的native(XLM)的contractAddress填写为native
示例： instance为config实例
instance.setTokenConfig("Ooxx", "1000005786703", "Ooxx/GCHH22AXHDDXET47Q3YKSANZ74GAAWEIMECQ3ZNFVM3SJY2LGYYPFLUH", 7, 6, "", "")
instance.setTokenConfig("XLM", "1000005786703", "native", 7, 6, "", "")
>4)stellar上创建anyToken时 需要2个mpc 作为冷热账户
https://developers.stellar.org/docs/issuing-assets/how-to-issue-an-asset/
步骤为：
1.B mpc 创建trustline 金额 = 初始金额
2.A mpc 转账给 B mpc 初始金额


## stellar测试步骤
**evm部署步骤这里不做赘述**

**// 所有的跨出memo写法统一为 bindAddr:toChainId**
>1)stellar原生货币XLM
```shell
1. 跨出
    1.1 stellar链上发起Payment方法，接收对象为mpc地址, memo使用buildSwapMemo工具生成,详见工具1.4
    {
        SourceAccount:        account,
        IncrementSequenceNum: true,
        BaseFee:              txnbuild.MinBaseFee,
        Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
        Memo:                 memo,
        Operations: []txnbuild.Operation{
            &txnbuild.Payment{
                Destination: toAddress,
                Amount:      amount,
                Asset:       asset,
            },
        },
    }
    1.2 注册交易
2. 跨入
    2.1 源联上发起跨出anyswapout交易
    function anySwapOut(address token, string memory to, uint amount, uint toChainID)
    2.2 注册交易
```

>2)stellar原生token
```shell
1 跨出
    1.1 stellar链上发起TrustSet方法，调用人为mpc，信任对象为code和issure, Limit为最大操作额度
    {
        SourceAccount:        mpc,
        IncrementSequenceNum: true,
        BaseFee:              txnbuild.MinBaseFee,
        Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
        Operations: []txnbuild.Operation{
            &txnbuild.ChangeTrust{
                Line:  asset,
                Limit: limitAmount,
            },
        },
    }
    1.2 stellar链上发起Payment方法，接收对象为mpc地址
    {
        SourceAccount:        account,
        IncrementSequenceNum: true,
        BaseFee:              txnbuild.MinBaseFee,
        Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
        Memo:                 memo,
        Operations: []txnbuild.Operation{
            &txnbuild.Payment{
                Destination: mpc,
                Amount:      amount,
                Asset:       asset,
            },
        },
    }
    1.3 注册交易
2 跨入
    2.1 源联上发起跨出anyswapout交易，bind字段需要特别注意
    function anySwapOut(address token, string memory to, uint amount, uint toChainID)
    // 如果目标用户有设置tfRequireDestTag，则为bindAddr:tag
    // 如果目标用户没有设置tfRequireDestTag，则为bindAddr
    2.2 注册交易
``` 

>3)stellar没有token，发行anyToken
```shell
1 发行anyToken
   1.1 // 同原生token 1.1
   1.2 // 同原生token 1.2 SourceAccount也是mpc
2 跨出
    // 同原生token跨出流程
3 跨入
    // 同原生token跨入流程
```

## stellar 工具调用方法
>1)golang
```shell
脚本路径： https://github.com/anyswap/CrossChain-Router/tree/feature/stellar/tokens/stellar/tools
1.1 getChainID
    go run ./tokens/stellar/tools/getStubChainID/main.go -p testnet|mainnet
1.2 address to publickey
    go run ./tokens/stellar/tools/addressToPublickey/main.go -a address
1.3 publickey to address
    go run ./tokens/stellar/tools/publicKeyToAddress/main.go -p publickey
1.4 build swapout tx memo
    go run ./tokens/stellar/tools/buildSwapMemo/main.go -a address -c chainID
1.5 trustLine 
    go run ./tokens/stellar/tools/sendTrustLineTx/main.go -config ./build/bin/config.toml -chainID chainID -priKey edPriKey -pubkey ecPublicKey -limitAmount 10000 -assetCode assetCode -issuer assetIssuer
    //mpc调用的话，去掉priKey参数
    go run ./tokens/stellar/tools/sendTrustLineTx/main.go -config ./build/bin/config-mpc.toml -chainID 5777 -pubkey 146f71db711bc259176f9bcba1756308d2a7af0f1c0b90deece65997a84c8f56 -limitAmount 100000 -assetCode Ooxx -issuer SBG6Z5SAGW2HRMGOMCQUHQTLE6MG7BR6O3AH2FCCBL5WEO5DORSSY2XB
    //测试网txhash: 6f3f4a552f3d3239ec1b5c62d4ed0e848003ad45a503a5ad03753c4cb86a964e
1.6 payment XLM/Token  
    go run ./tokens/stellar/tools/sendPaymentTx/main.go -config config.toml -chainID chainID -priKey edPriKey -pubkey edPublicKey -destination receiptAddr -amount 100000000 -assetCode assetCode -issuer assetIssuer
    //mpc调用的话，去掉priKey参数
    go run ./tokens/stellar/tools/sendPaymentTx/main.go -config ./build/bin/config-mpc.toml -chainID chainid -pubkey edPublicKey -destination receiptAddr -amount 100000000 -assetCode Ooxx -issuer SBG6Z5SAGW2HRMGOMCQUHQTLE6MG7BR6O3AH2FCCBL5WEO5DORSSY2XB
    //测试网txhash: 3a0bca0aae8fa908f4eb16605ae17cf8fdec701c7b2e92d858ce03f9ac0c2694
```
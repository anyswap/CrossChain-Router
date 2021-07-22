# CrossChain-Router API

[JSON RPC API Reference](#json-rpc-api-reference)

[RESTful API Reference](#restful-api-reference)

## JSON RPC API Reference

[swap.RegisterRouterSwap](#swapregisterrouterswap)  
[swap.GetRouterSwap](#swapgetrouterswap)  
[swap.GetRouterSwapHistory](#swapgetrouterswaphistory)  
[swap.GetVersionInfo](#swapgetversioninfo)  
[swap.GetServerInfo](#swapgetserverinfo)  
[swap.GetAllChainIDs](#swapgetallchainids)  
[swap.GetAllTokenIDs](#swapgetalltokenids)  
[swap.GetAllMultichainTokens](#swapgetallmultichaintokens)  
[swap.GetChainConfig](#swapgetchainconfig)  
[swap.GetTokenConfig](#swapgettokenconfig)  
[swap.GetSwapConfig](#swapgetswapconfig)  

### swap.RegisterRouterSwap

注册置换交易

##### 参数：
```json
[{"chainid":"链ChainID", "txid":"交易哈希", "logindex":"日志下标"}]
```
其中 logindex 为可选参数，对应日志下标，默认值为 0。
如果 logindex 为 0, 则注册本交易中的所有置换。

##### 返回值：
```text
成功返回`Success`，失败返回错误。
```

### swap.GetRouterSwap

查询置换状态

##### 参数：
```json
[{"chainid":"链ChainID", "txid":"交易哈希", "logindex":"日志下标"}]
```
其中 logindex 为可选参数，对应日志下标，默认值为 0。
如果 logindex 为 0, 则自动查询本交易中的第一个置换。

##### 返回值：
```text
成功返回置换状态，失败返回错误。
```

### swap.GetRouterSwapHistory

查询置换历史，支持分页，addess 为账户地址

##### 参数：
```json
[{"chainid":"链ChainID", "address":"账户地址", "offset":"偏移量", "limit":"数量限制", "status":"8,9"}]
```
其中 status 为可选参数，如果指定可以根据状态进行查询。
其中 offset，limit 为可选参数，默认值分别为 0 和 20。
如果 limit 为负数，表示按时间逆序排序后取结果。

##### 返回值：
```text
成功返回置换历史，失败返回错误。
```

### swap.GetVersionInfo

##### 参数：
```text
无
```

##### 返回值：
```text
返回版本号信息
```

### swap.GetServerInfo

##### 参数：
```text
无
```

##### 返回值：
```text
获取服务信息
```

### swap.GetAllChainIDs

##### 参数：
```text
无
```

##### 返回值：
```text
获取所有 chainID
```

### swap.GetAllTokenIDs

##### 参数：
```text
无
```

##### 返回值：
```text
获取所有 tokenID
```

### swap.GetAllMultichainTokens

##### 参数：
```json
["tokenID"]
```

##### 返回值：
```text
获取指定 tokenID 的所有 multichain token
```

### swap.GetChainConfig

##### 参数：
```json
["链ChainID"]
```

##### 返回值：
```text
获取指定 chainID 的 chain 配置
```

### swap.GetTokenConfig

##### 参数：
```json
[{"chainid":"链ChainID", "address":"账户地址"}]
```

##### 返回值：
```text
获取指定 chainID 和 token 地址的 token 配置
```

### swap.GetSwapConfig

##### 参数：
```json
[{"tokenid": "tokenID", "chainid":"目标链ChainID"}]
```

##### 返回值：
```text
获取指定 tokenID 和目标链 chainID 对应的 swap 配置
```


## RESTful API Reference

### POST /swap/register/{chainid}/{txid}?logindex=0

注册置换交易

其中 logindex 为可选参数，对应的日志下标，默认值为 0。
如果 logindex 为 0, 则注册本交易中的所有置换。

### GET /swap/status/{chainid}/{txid}?logindex=0

查询置换状态

其中 logindex 为可选参数，对应日志下标，默认值为 0。
如果 logindex 为 0, 则自动查询本交易中的第一个置换。

### GET /swap/history/{chainid}/{address}?offset=0&limit=20&status=8,9

查询置换历史，支持分页，addess 为账户地址

其中 status 为可选参数，如果指定可以根据状态进行查询。
其中 offset，limit 为可选参数，默认值分别为 0 和 20。
如果 limit 为负数，表示按时间逆序排序后取结果。

### GET /versioninfo
获取版本号信息

### GET /serverinfo
获取服务信息

### GET /allchainids
获取所有 chainID

### GET /alltokenids
获取所有 tokenID

### GET /allmultichaintokens/{tokenid}
获取指定 tokenID 的所有 multichain token

### GET /chainconfig/{chainid}
获取指定 chainID 的 chain 配置

### GET /tokenconfig/{chainid}/{address}
获取指定 chainID 和 token 地址的 token 配置

### GET /swapconfig/{tokenid}/{chainid}
获取指定 tokenID 和目标链 chainID 对应的 swap 配置

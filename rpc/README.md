# CrossChain-Router API

[JSON RPC API Reference](#json-rpc-api-reference)

[RESTful API Reference](#restful-api-reference)

## JSON RPC API Reference

[swap.RegisterRouterSwap](#swapregisterrouterswap)  
[swap.GetRouterSwap](#swapgetrouterswap)  
[swap.GetRouterSwapHistory](#swapgetrouterswaphistory)  
[swap.GetVersionInfo](#swapgetversioninfo)  
[swap.GetIdentifier](#swapgetidentifier)  

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
[{"chainid":"链ChainID", "address":"账户地址", "offset":"偏移量", "limit":"数量限制"}]
```
其中 offset，limit 为可选参数，默认值分别为 0 和 20。
如果 limit 为负数，表示按时间逆序排序后取结果。

##### 返回值：
```text
成功返回置换历史，失败返回错误。
```

### swap.GetVersionInfo

##### 返回值：
```text
返回版本号信息
```

### swap.GetIdentifier

##### 返回值：
```text
返回惟一标识
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

### GET /swap/history/{chainid}/{address}?offset=0&limit=20

查询置换历史，支持分页，addess 为账户地址

其中 offset，limit 为可选参数，默认值分别为 0 和 20。
如果 limit 为负数，表示按时间逆序排序后取结果。

### GET /versioninfo
获取版本号信息

### GET /identifier
获取惟一标识

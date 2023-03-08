# bridge wrapper

bridge wrapper is a wrapper implementation to separate router and chain support endpoints.

the chain support endpoints must implement required rpc methods.

startup chain support endpoints firstly, and then run router program.

## router gateway configs

```toml
[WrapperGateways.1000000000000]
SupportNonce = true
RPCTimeout = 20
RPCAddress = "XXXXXX"
```

## chain support endpoint configs

```toml
ChainID = "1000000000000"

# router config
RouterConfigFile = "XXXXXX"
InitRouterServer = false

# listen port
Port = 12556

[GatewayConfig]
APIAddress = ["https://xxxx.xxx"]
APIAddressExt = []
GRPCAPIAddress = []
```

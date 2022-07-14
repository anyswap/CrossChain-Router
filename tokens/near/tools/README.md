## tools for call to near by mpc publicKey

### changeMpc
> go run functionCall/main.go -config config.toml -chainID xxx -routerContract xxx -functionName change_mpc_id -newMpcId xxx -pubKey xxx

### setBaseGas
> go run functionCall/main.go -config config.toml -chainID xxx -routerContract xxx -functionName set_base_gas -newGas xxx -pubKey xxx

### setGas
> go run functionCall/main.go -config config.toml -chainID xxx -routerContract xxx -functionName set_gas -newGas xxx -token xxx -pubKey xxx

### anySwapInAll
> go run functionCall/main.go -config config.toml -chainID xxx -routerContract xxx -functionName any_swap_in_all -txHash xxx -token xxx -to xxx -amount xxx -fromChainId xxx -pubKey xxx

### changeWnative
> go run functionCall/main.go -config config.toml -chainID xxx -routerContract xxx -functionName change_wnative -newWnative xxx -pubKey xxx

### sendNear
> go run sendNear/main.go -config config.toml -chainID xxx -to xxx -amount xxx -pubKey xxx



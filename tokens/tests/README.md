# Description

This package is used to simplify the developing process to support `CrossChain-Router` project.

It's designed to get rid of business related processes: MPC and Database.

So the developers can focus on the aimed blockchain related interfaces.

## How to do tests for a new block chain

1. create a new directory to start work

for example, create `tokens/tests/eth/` directory by copying from `tokens/tests/template/`

change the package name accordingly after copied, and pass the building.

2. prepare test enviroment and construct a transaction to test

choose or run a full node to synchronize blocks and do rpc queries.

deploy smart contracts (`RouterContract` and `TokenContract`).

construct and send a swapout tx by calling `RouterContract`'s function (`anySwapOut`, `anySwapOutUnderlying` or `anySwapOutNative`)

3. implement interfaces in our new directory

add new source files, functions, structs, variables, and config items to implement the interfaces (verify, build, sign and send tx).

for example, we can keep the `bridge.go` file concise, and do the implementations in new files.

add new test module initialization in `initRouter` function in file `tokens/tests/main.go`

4. modify the config file

copy the `tokens/tests/config/config-test.toml` template config file, and modify the config items.

5. testing and debuging

run the test program (exit with `Ctrl+C`)

```shell
go run tokens/tests/main.go -c <config-file>
```

in another terminal trigger the testing

```shell
# url format is "http://{host}:{port}/swap/test/{txhash}"
curl -sS http://127.0.0.1:11556/swap/test/0xcea2be8a05c0155832676e89c129b785fd1e2f308439606fc5df98a0e133bff2
```

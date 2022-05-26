# How to deploy router swap

## 0. compile

```shell
make all
```
run the above command, it will generate `./build/bin/swaprouter` binary.

## 1. deploy `AnyswapRouter`

deploy a `AnyswapRouter` contract for each supported blockchain

eg. [AnyswapV6Router.sol](https://github.com/anyswap/anyswap-v1-core/blob/master/contracts/AnyswapV6Router.sol)

## 2. deploy `AnyswapERC20`

deploy a `AnyswapERC20` contract for each token on each blockchain

eg. [AnyswapV6ERC20.sol](https://github.com/anyswap/anyswap-v1-core/blob/master/contracts/AnyswapV6ERC20.sol)

eg. [AnyswapV6ERC20-NonEVM.sol](https://github.com/anyswap/anyswap-v1-core/blob/master/contracts/AnyswapV6ERC20-NonEVM.sol)

## 3. deploy `RouterConfig`

deploy a `RouterConfig` contract to store router bridge configs

eg. [RouterConfigV2.sol](https://github.com/anyswap/router-config/blob/main/contracts/RouterConfigV2.sol)

## 4. set router config on chain

call `RouterConfig` contract to set configs on blcokchain.

The following is the most used functions, please ref. the abi for more info.

### 4.1 set chain config

call the following contract function:

```solidity
function setChainConfig(uint256 chainID, string blockChain, string routerContract, uint64 confirmations, uint64 initialHeight, string extra)
```

### 4.2 set token config

call the following contract function:

```solidity
function setTokenConfig(string tokenID, uint256 chainID, string tokenAddr, uint8 decimals, uint256 version, string routerContract, string extra)
```

### 4.3 set swap and fee config

we may not set swap and fee config between all fromChainIDs and toChainIDs,
and we'll use the following logic to decide the actual config to use.

```text
the actual swap config is decided by the following steps
1. if _swapConfig[tokenID][srcChainID][dstChainID] exist, then use it.
2. else if _swapConfig[tokenID][srcChainID][0] exist, then use it.
3. else if _swapConfig[tokenID][0][dstChainID] exist, then use it.
4. else use _swapConfig[tokenID][0][0].
```

```text
the actual fee config is decided by the following steps
1. if _feeConfig[tokenID][srcChainID][dstChainID] exist, then use it.
2. else if _feeConfig[tokenID][srcChainID][0] exist, then use it.
3. else if _feeConfig[tokenID][0][dstChainID] exist, then use it.
4. else use _feeConfig[tokenID][0][0].
```

1. set swap and fee config in batch

```solidity
    struct SwapConfig2 {
        uint256 FromChainID;
        uint256 ToChainID;
        uint256 MaximumSwap;
        uint256 MinimumSwap;
        uint256 BigValueThreshold;
    }

    struct FeeConfig2 {
        uint256 FromChainID;
        uint256 ToChainID;
        uint256 MaximumSwapFee;
        uint256 MinimumSwapFee;
        uint256 SwapFeeRatePerMillion;
    }

    function setSwapConfigs(string memory tokenID, SwapConfig2[] calldata configs)
    function setFeeConfigs(string memory tokenID, FeeConfig2[] calldata configs)
```

2. query swap and fee config in batch

```solidity
    function getAllSwapConfigs(string memory tokenID) external view returns (SwapConfig2[] memory)
    function getAllFeeConfigs(string memory tokenID) external view returns (FeeConfig2[] memory)
```

#### 4.3.1 set swap and fee config meantime

call the following contract function:

```solidity
function setSwapAndFeeConfig(
        string tokenID, uint256 srcChainID, uint256 dstChainID,
        uint256 maxSwap, uint256 minSwap, uint256 bigSwap,
        uint256 maxFee, uint256 minFee, uint256 feeRate)
```

#### 4.3.2 set swap config alone

call the following contract function to set swap config:

max/min/big value always uses decimals 18 (like precision).

```solidity
function setSwapConfig(string tokenID, uint256 srcChainID, uint256 dstChainID, uint256 maxSwap, uint256 minSwap, uint256 bigSwap)
```

swap config is stored in a map with keys tokenID,srcChainID,dstChainID
```solidity
mapping (bytes32 => mapping(uint256 => mapping(uint256 => SwapConfig))) private _swapConfig;
```

#### 4.3.3 set fee config alone

call the following contract function:

rate is per million ration.

max/min value always uses decimals 18 (like precision).

```solidity
function setFeeConfig(string tokenID, uint256 srcChainID, uint256 dstChainID, uint256 maxFee, uint256 minFee, uint256 feeRate)
```

fee config is stored in a map with keys tokenID,srcChainID,dstChainID
```solidity
mapping (bytes32 => mapping(uint256 => mapping(uint256 => FeeConfig))) private _feeConfig;
```

### 4.4 set mpc address's public key

call the following contract function:

```solidity
function setMPCPubkey(address addr, string pubkey);
```

## 5. add local config file

please ref. [config-example.toml](https://github.com/anyswap/CrossChain-Router/blob/main/params/config-example.toml)

the following is the major config items:

| config item | description |
| --------------- | --------------- |
| Identifier | must have prefix 'routerswap' |
| SwapType | eg. erc20swap, nftswap, anycallswap |
| SwapSubType | anycall has subtype of v5 and v6 |
| [Server] | only need by swap server |
| [Server.MongoDB] | use mongodb database |
| [Server.APIServer] | provide rpc service |
| [Oracle] | only need by swap oracle |
| [Extra] | extra configs |
| [OnChain] | get onchain router configs in samrt contract |
| [Gateways] | query block and tx info from full nodes' rpc |
| [MPC] | use mpc to do threshold signing |

**Notation**:

the `MPC` is a security Multi-Party threshold Computation,
for more info, please refer [FastMulThreshold-DSA](https://github.com/anyswap/FastMulThreshold-DSA)

Because the complexity of `MPC`, we can use private key to sign tx for easy testing.

set the following config items in the `[MPC]` section:

```toml
[MPC]
# use private key instead (use for testing)
SignWithPrivateKey = true

# set signer's private key, key is chain ID (use for testing)
[MPC.SignerPrivateKeys]
4  = "1111111111111111111111111111111111111111111111111111111111111111"
97 = "2222222222222222222222222222222222222222222222222222222222222222"
```

for more info, please ref. [config-sign-with-privatekey-example.toml](https://github.com/anyswap/CrossChain-Router/blob/main/params/config-sign-with-privatekey-example.toml)

## 6. run swaprouter

```shell
# for server run (add '--runserver' option)
setsid ./build/bin/swaprouter --config config.toml --log logs/routerswap.log --runserver

# for oracle run
setsid ./build/bin/swaprouter --config config.toml --log logs/routerswap.log
```

## 7. sub commands

get all sub command list and help info, run

```shell
./build/bin/swaprouter -h
```

sub commands:

`admin` is admin tool

`config` is tool to process and query config data

## 8. RPC api

please ref. [server rpc api](https://github.com/anyswap/CrossChain-Router/blob/main/rpc/README.md)

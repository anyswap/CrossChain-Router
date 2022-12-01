# reef router

## github
https://github.com/anyswap/CrossChain-Router/tree/feature/reef-substrate

## router contract
https://github.com/anyswap/anyswap-v1-core


## router mechanism
```mermaid
graph TB
    begin(mpc public key)--> IsRemaining{"generate"}
    IsRemaining -->|reef|reefAddr[ss58address]--> done(done)
    IsRemaining --evm--> evmAddr[evmAddr]--> done
```


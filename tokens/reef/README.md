# reef router

## github
https://github.com/anyswap/CrossChain-Router/tree/feature/reef-substrate

## router contract
https://github.com/anyswap/anyswap-v1-core


## router mechanism
```mermaid
graph TB
    begin(出门)--> buy[买炸鸡]
    buy --> IsRemaining{"还有没有炸鸡？"}
    IsRemaining -->|有|happy[买完炸鸡开心]--> goBack(回家)
    IsRemaining --没有--> sad["伤心"]--> goBack
```


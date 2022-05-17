# How to research a blockchain and add cross-chain bridge or router supporting

## 1. retrieve blockchain resources

```text
blockchain full nodes (mainnet, testnet, devnet, etc.)

blockchain basics (chainid, consensus, algorithm of signing(ec256k1, ed25519, etc.), address and public key format(hex, base58, etc.))

blockchain explorer (verifing smart contracts)

developer documents (concept, rpc api, etc.)

sdk tools (nodejs, golang, rust, c++, java, etc.)

wallet usage (metamask, browser, plugin, app, etc.)

blockchain ecology (supported exchanges, tvl, etc.)

commnication channels (wecahat, telagram, facebook, twitter, email, etc.)
```

## 2. add cross-chain bridge or router supporting

### 2.1 verify swapout tx from this blockchain

common considerations

```text
initial height (ignore tx before initial height)

stable height (tx maybe rollbacked)

success status (tx myabe failed)

filtered logs (tx maybe have no corresponding logs)
```

### 2.2 send swapin tx to this blockchain

common considerations

```text
account nonce or sequence (if support, then use it to prevent duplicate sending tx)

algorithm of signing (ec, ed, etc.)

sign content (message hash, message content, messge content with prefix or suffix, etc.)

mpc sign public key (we may need convert blockchain specifically encoded public key to mpc sign public key)

calc signed transaction hash (calc offline instead of get result from rpc calling as rpc maybe timeout)
```

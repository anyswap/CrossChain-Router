# How to research a blockchain and add cross-chain bridge or router supporting

## 1. retrieve blockchain resources

```text
blockchain full nodes (mainnet, testnet, devnet, etc.)

blockchain basics
	chainid (some blockchains has no chainID, we can assign them a stub one)
	consensus (PoW, PoS, PoA, etc.)
	algorithm of signing(ec256k1, ed25519, etc.)
	address and public key format(hex, base58, etc.))
		storage model
		balance query
	native token (eg. ETH)
	system token/asset (eg. ERC20, BEP20, etc.)
		standard interface
		balance query
	cross-chain mechanism
		use memo (specify a custom memo in deposit transaction)
		use smart contracts (AnyswapRouter, AnyswapERC20, or others)
	smart contract
		is smart contract supportable?
		smart contract development language: solidity, wasm or others?
		how to write custom smart contract like AnyswapRouter and AnyswapERC20?
			owner/minter
			mint/burn
			deposit/withdraw
			swapin/swapout
		how to deploy smart contract?
		how to use rpc calling to get contract infos (eg. decimals, underlying, etc)?
		how to generate cross-chain related logs, and how to verify these logs?


blockchain explorer (verifing smart contracts)

developer documents (concept, rpc api, etc.)

sdk tools (nodejs, golang, rust, c++, java, etc.)

wallet usage (metamask, browser, plugin, app, etc.)

blockchain ecology (supported exchanges, dex exchanges, uniswap, exist cross-chain projects, tvl, etc.)

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

parse memos if use memo cross-chain mechanism
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

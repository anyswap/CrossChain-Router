#### run cardano node

```text
https://developers.cardano.org/docs/get-started/installing-cardano-node/#overview
```

#### minting native tokenclear

```text
https://docs.cardano.org/native-tokens/getting-started
```

## deploy step

```
1 get mpc publickKey and write a json file with publicKey to build a payment address
cardano-cli address build --payment-verification-key-file mpc.vkey --out-file mpc.addr --testnet-magic 1097911063

mpc.vkey:
{
    "type": "PaymentVerificationKeyShelley_ed25519",
    "description": "Payment Verification Key",
    "cborHex": "5820[mpc publicKey]"
}

2 set config contract
	2.1 routerAddress is mpc address
	2.2 tokenAddress is assedId

3 chainId
mainnet: 1645027868239
testnet: 1645027868240

4 url
mainnet: https://graphql-api.mainnet.dandelion.link/
testnet: https://graphql-api.testnet.dandelion.link/

5 update NetWork fiels in ./tokens/cardano/rpcClient.go
testnet(1.35.2): --testnet-magic 1097911063
testnet(1.35.3): --testnet-magic 1

```

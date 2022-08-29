#### run cardano node

```text
https://developers.cardano.org/docs/get-started/installing-cardano-node/#overview
```

#### minting native tokenclear

```text
https://docs.cardano.org/native-tokens/getting-started
```

## deploy step

```text
cardano-cli transaction policyid --script-file ./policy/policy.script 
328a60495759e0d8e244eca5b85b2467d142c8a755d6cd0592dff47bcardano-cli transaction policyid --script-file ./policy/policy.script 
328a60495759e0d8e244eca5b85b2467d142c8a755d6cd0592dff47b1 update NetWork fiels in ./tokens/cardano/rpcClient.go
testnet(1.35.2): --testnet-magic 1097911063
testnet(1.35.3): --testnet-magic 1
mainnet:	 --mainnet

2 get mpc publickKey and write a json file with publicKey to build a payment address
cardano-cli address build --payment-verification-key-file mpc.vkey --out-file mpc.addr --testnet-magic 1097911063

mpc.vkey:
{
    "type": "PaymentVerificationKeyShelley_ed25519",
    "description": "Payment Verification Key",
    "cborHex": "5820[mpc publicKey]"
}

3 set config contract
	3.1 routerAddress is mpc address
	3.2 tokenAddress is assetId.assetName if token not ADA
	    tokenAddress is lovelace if token is ADA

4 chainId
mainnet: 1645027868239
testnet: 1645027868240

5 url
mainnet: https://graphql-api.mainnet.dandelion.link/
testnet: https://graphql-api.testnet.dandelion.link/

6 update protocol.json file in ./txDb/config/protocol.json
cardano-cli  query protocol-parameters \
--testnet-magic 1097911063 \
--out-file protocol.json

7 mint native token
need register metadata to repo: https://github.com/cardano-foundation/cardano-token-registry
	7.1 create policy.script(cp script file to ./txDb/policy)
	touch policy/policy.script && echo "{" > policy/policy.script 
	echo "  \"keyHash\": \"$(cardano-cli address key-hash --payment-verification-key-file 		mpc.vkey)\"," >> policy/policy.script 
	echo "  \"type\": \"sig\"" >> policy/policy.script 
	echo "}" >> policy/policy.script

	7.2 create policyId(update policyId field in ./tokens/cardano/cardanoCmd.gon)
	cardano-cli transaction policyid --script-file ./policy/policy.script 

	7.3 create assetNameId
	echo -n "tokenName" | xxd -ps
```

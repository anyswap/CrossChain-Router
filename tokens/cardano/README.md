#### run cardano node

```text
https://developers.cardano.org/docs/get-started/installing-cardano-node/#overview
```

#### Graphql

```text
https://github.com/input-output-hk/cardano-graphql
```

#### minting native tokenclear

```text
https://docs.cardano.org/native-tokens/getting-started
```

## deploy step

```text
1 chainId
mainnet: 1645027868239
testnet: 1645027868240

2 url
mainnet: https://graphql-api.mainnet.dandelion.link/
testnet: https://graphql-api.testnet.dandelion.link/

3 update protocol.json file in ./txDb/config/protocol.json
cardano-cli query protocol-parameters \
--testnet-magic 1097911063 \
--out-file protocol.json

4 update NetWork fiels in tokens/cardano/cardanoCmd.go
testnet(1.35.2): --testnet-magic 1097911063
testnet(1.35.3): --testnet-magic 1
mainnet:	 --mainnet

5 get mpc publickKey and write a json file with publicKey to build a payment address
cardano-cli address build --payment-verification-key-file mpc.vkey --out-file mpc.addr --testnet-magic 1097911063

mpc.vkey:
{
    "type": "PaymentVerificationKeyShelley_ed25519",
    "description": "Payment Verification Key",
    "cborHex": "5820[mpc publicKey]"
}

6 mint native token
	need register metadata to repo: https://github.com/cardano-foundation/cardano-token-registry

	6.1 create policy.script(cp script file to ./txDb/policy)

	touch policy/policy.script && echo "{" > policy/policy.script 
	echo "  \"keyHash\": \"$(cardano-cli address key-hash --payment-verification-key-file mpc.vkey)\"," >> policy/policy.script 
	echo "  \"type\": \"sig\"" >> policy/policy.script 
	echo "}" >> policy/policy.script

	cat policy/policy.script
	res: 
	{
  		"keyHash": "b930621762bcb49f2735540974d5703d55729582eb31b9036c04ad24",
  		"type": "sig"
	}

	6.2 create policyId(update policyId field in ./tokens/cardano/cardanoCmd.gon)

	cardano-cli transaction policyid --script-file ./policy/policy.script 
	res: 8c7d04a9146bff45aadc19d3f3e4cdb8a989ff35182989dbfb93f5b4

	6.3 create assetNameId

	echo -n "tokenName" | xxd -ps
	res: 746f6b656e4e616d65

7 set config contract
	7.1 routerAddress is mpc address
	7.2 tokenAddress is policyId.assetName if token not ADA
	    tokenAddress is lovelace if token is ADA
```

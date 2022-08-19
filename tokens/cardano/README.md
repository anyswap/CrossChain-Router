#### run cardano node

```text
https://developers.cardano.org/docs/get-started/installing-cardano-node/#overview
```

#### minting native token

```text
https://docs.cardano.org/native-tokens/getting-started
```

## publicKey to addr

```
cardano-cli address build --payment-verification-key-file mpc.vkey --out-file mpc.addr --testnet-magic 1097911063

mpc.vkey:
{
    "type": "PaymentVerificationKeyShelley_ed25519",
    "description": "Payment Verification Key",
    "cborHex": "5820[mpc publicKey]"
}
```

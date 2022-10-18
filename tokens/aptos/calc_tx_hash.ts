import { BCS, TxnBuilderTypes, TransactionBuilder, HexString } from "aptos";

async function main() {

    process.argv.forEach((val, index) => {
        console.log(`${index}: ${val}`);
    });
    // #############
    // let json = "{\"sender\":\"0x06da2b6027d581ded49b2314fa43016079e0277a17060437236f8009550961d6\",\"sequence_number\":\"41\",\"max_gas_amount\":\"100000\",\"gas_unit_price\":\"1000\",\"expiration_timestamp_secs\":\"1666000058\",\"payload\":{\"type\":\"entry_function_payload\",\"function\":\"0x06da2b6027d581ded49b2314fa43016079e0277a17060437236f8009550961d6::wETH::mint\",\"type_arguments\":[],\"arguments\":[\"0x10878abd3802be00d674709b1e5554488823f5f825bce8d1efaf370e9aaac777\",\"100000000000000000\"]}}"
    // let argumentsTyps = ["address", "uint64"]
    // #############
    let json = process.argv[2]
    console.log(json)

    let argumentsTyps = process.argv[3].split(",")
    console.log(argumentsTyps)

    let chainId = Number(process.argv[4])
    console.log(chainId)


    let originTX = JSON.parse(json)
    console.log(originTX)
    let payload = originTX.payload

    let ty_args: TxnBuilderTypes.TypeTagStruct[] = new Array()
    for (let index = 0; index < payload.type_arguments.length; index++) {
        let ta = new TxnBuilderTypes.TypeTagStruct(
            TxnBuilderTypes.StructTag.fromString(payload.type_arguments[index]),
        );
        ty_args.push(ta)
    }
    // console.log(ty_args)

    let args: Uint8Array[] = new Array()
    for (let index = 0; index < payload.arguments.length; index++) {
        let type = argumentsTyps[index]
        let value = payload.arguments[index]
        switch (type) {
            case "string":
                args.push(BCS.bcsSerializeStr(value))
                console.log(type, value, HexString.fromUint8Array(BCS.bcsSerializeStr(value)).hex())
                break;
            case "uint64":
                let bi = BigInt(value)
                args.push(BCS.bcsSerializeUint64(value))
                console.log(type, bi, HexString.fromUint8Array(BCS.bcsSerializeUint64(bi)).hex())
                break;
            case "address":
                args.push(BCS.bcsToBytes(TxnBuilderTypes.AccountAddress.fromHex(value)))
                console.log(type, value, HexString.fromUint8Array(BCS.bcsToBytes(TxnBuilderTypes.AccountAddress.fromHex(value))).hex())
                break;
            default:
                break;
        }
    }


    if (payload.type == "entry_function_payload") {

        console.log("########### entry_function_payload ###########")

        let modules = payload.function.split("::")
        const scriptFunctionPayload = new TxnBuilderTypes.TransactionPayloadEntryFunction(
            TxnBuilderTypes.EntryFunction.natural(
                modules[0] + "::" + modules[1],
                modules[2],
                ty_args,
                args
            ),
        );


        const rawTxn = new TxnBuilderTypes.RawTransaction(
            TxnBuilderTypes.AccountAddress.fromHex(originTX.sender),
            BigInt(originTX.sequence_number),
            scriptFunctionPayload,
            BigInt(originTX.max_gas_amount),
            BigInt(originTX.gas_unit_price),
            BigInt(originTX.expiration_timestamp_secs),
            new TxnBuilderTypes.ChainId(chainId),
        );


        const signingMessage = TransactionBuilder.getSigningMessage(rawTxn);
        // console.log("signingMessage", HexString.fromBuffer(signingMessage).hex())

        const sigHexStr = originTX.signature.signature
        // console.log("sigHexStr", sigHexStr.hex())
        const signature = new TxnBuilderTypes.Ed25519Signature(new HexString(sigHexStr).toUint8Array());
        const authenticator = new TxnBuilderTypes.TransactionAuthenticatorEd25519(
            new TxnBuilderTypes.Ed25519PublicKey(new HexString(originTX.signature.public_key).toUint8Array()),
            signature
        );
        const signedTx = new TxnBuilderTypes.SignedTransaction(rawTxn, authenticator);

        // console.log("signedTx hex", HexString.fromBuffer(bcs.getBytes()).hex())
        const userTransaction = new TxnBuilderTypes.UserTransaction(signedTx)

        // console.log("signedTx hash", HexString.fromBuffer(userTransaction.hash()).hex())

        return HexString.fromBuffer(userTransaction.hash()).hex()
    }

}

main().then((resp) => console.log(resp));

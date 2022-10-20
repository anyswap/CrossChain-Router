package main

import (
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens/aptos"
)

func main() {
	aptos.InstallTsModules()

	txbody := `{"sender":"0x06da2b6027d581ded49b2314fa43016079e0277a17060437236f8009550961d6","sequence_number":"58","max_gas_amount":"100000","gas_unit_price":"1000","expiration_timestamp_secs":"1666244737","payload":{"type":"entry_function_payload","function":"0x06da2b6027d581ded49b2314fa43016079e0277a17060437236f8009550961d6::wETH::mint","type_arguments":[],"arguments":["0x10878abd3802be00d674709b1e5554488823f5f825bce8d1efaf370e9aaac777","100000000000000000"]},"signature":{"type":"ed25519_signature","public_key":"0x11e202042f518e9bd719296fa36007017948392f6557d2796c81677620e5a4a4","signature":"0xd3934d202a9de3178e9b280fdcfd614bb9f82d2ffd0e305898f483cdf48cf67c8350451147a5a6644d590f0a18892b12af37f47de46dd5c44ed7e2183865180b"}}`

	argTypes := `address,uint64`

	chainId := uint(2)

	res, err := aptos.RunTxHashScript(&txbody, &argTypes, chainId)
	if err != nil {
		log.Fatal("RunTxHashScript failed", "err", err)
	}
	log.Infof("RunTxHashScript success. txHash is %v", res)
}

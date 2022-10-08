package cardano

import (
	"bytes"
	"math/big"
	"os/exec"
	"strings"
)

const (
	NetWork       = "--testnet-magic 1"
	MPCPolicyId   = "8ce5ab9f1216a7559e14ab8e2dd7af4dcc34c7e718fe239902993586"
	RawPath       = "txDb/raw/"
	AdaAsset      = "lovelace"
	RawSuffix     = ".raw"
	WitnessPath   = "txDb/witness/"
	WitnessSuffix = ".witness"
	WitnessType   = "TxWitness AlonzoEra"
	SignedPath    = "txDb/signed/"
	SignedSuffix  = ".signed"
)

var (
	AddressInfoCmd           = "cardano-cli address info --address %s"
	AssembleCmd              = "cardano-cli transaction assemble --tx-body-file %s --witness-file %s --out-file %s"
	SubmitCmd                = "cardano-cli transaction submit --tx-file %s " + NetWork
	FixAdaAmount             = big.NewInt(1500000)
	DefaultAdaAmount         = big.NewInt(2000000)
	BuildRawTxWithoutMintCmd = "cardano-cli  transaction  build-raw  --fee  %s%s%s  --out-file  %s"
	BuildRawTxWithMintCmd    = "cardano-cli  transaction  build-raw  --fee  %s%s%s%s  --out-file  %s  --mint-script-file  txDb/policy/policy.script"
	CalcMinFeeCmd            = "cardano-cli transaction calculate-min-fee --tx-body-file %s --tx-in-count %d --tx-out-count %d --witness-count 1 --protocol-params-file txDb/config/protocol.json " + NetWork
	CalcTxIdCmd              = "cardano-cli transaction txid --tx-body-file %s"
	QueryTipCmd              = "cardano-cli query tip " + NetWork
	QueryTransaction         = "{transactions(where: { hash: { _eq: \"%s\"}}) {block {number epochNo slotNo}hash metadata{key value} outputs(order_by:{index:asc}){address index tokens{ asset{policyId assetName}quantity}value}validContract}}"
	QueryOutputs             = "{utxos(where: { address: { _eq: \"%s\"}}) {txHash index tokens {asset {policyId assetName} quantity} value}}"
	TransactionChaining      = &TransactionChainingMap{InputKey: UtxoKey{}, AssetsMap: make(map[string]string)}
)

func ExecCmd(cmdStr, space string) (string, error) {
	list := strings.Split(cmdStr, space)
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		return "", err
	} else {
		return cmdOut.String(), nil
	}
}
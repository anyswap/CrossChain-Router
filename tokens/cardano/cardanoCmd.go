package cardano

import (
	"bytes"
	"math/big"
	"os/exec"
	"strings"
)

const (
	NetWork    = "--testnet-magic 1"
	PolicyId   = "8c7d04a9146bff45aadc19d3f3e4cdb8a989ff35182989dbfb93f5b4"
	RawPath    = "txDb/raw/"
	AdaAssetId = "lovelace"
	RawSuffix  = ".raw"
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
	QueryUtxoCmd             = "cardano-cli query utxo --address %s " + NetWork
	CalcTxIdCmd              = "cardano-cli transaction txid --tx-body-file %s"
	QueryTipCmd              = "cardano-cli query tip " + NetWork
	QueryMethod              = "{transactions(where: { hash: { _eq: \"%s\"}}) {block {number epochNo}hash metadata{key value}inputs{tokens{asset{ assetId assetName}quantity }value}outputs{address tokens{ asset{assetId assetName}quantity}value}validContract}}"
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

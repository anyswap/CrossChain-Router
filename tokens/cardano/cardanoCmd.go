package cardano

import (
	"bytes"
	"math/big"
	"os/exec"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	NetWork       = "--mainnet" // for testnet, use "--testnet-magic 1"
	MPCPolicyId   = "f73d0275b986a17537f0dfa94060313f922ac1d7a81aec677fa3bdbe"
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
	FixAdaAmount                = big.NewInt(1500000)
	DefaultAdaAmount            = big.NewInt(2000000)
	AddressInfoCmd              = "cardano-cli address info --address %s"
	AssembleCmd                 = "cardano-cli transaction assemble --tx-body-file %s --witness-file %s --out-file %s"
	SubmitCmd                   = "cardano-cli transaction submit --tx-file %s " + NetWork
	BuildRawTxWithoutMintCmd    = "cardano-cli  transaction  build-raw  --fee  %s%s%s  --out-file  %s"
	BuildRawTxWithMintCmd       = "cardano-cli  transaction  build-raw  --fee  %s%s%s%s  --out-file  %s  --mint-script-file  txDb/policy/policy.script"
	CalcMinFeeCmd               = "cardano-cli transaction calculate-min-fee --tx-body-file %s --tx-in-count %d --tx-out-count %d --witness-count 1 --protocol-params-file txDb/config/protocol.json " + NetWork
	CalcTxIdCmd                 = "cardano-cli transaction txid --tx-body-file %s"
	QueryTipCmd                 = "cardano-cli query tip " + NetWork
	QueryTransaction            = "{transactions(where: { hash: { _eq: \"%s\"}}) {block {number epochNo slotNo}hash metadata{key value} outputs(order_by:{index:asc}){address index tokens{ asset{policyId assetName}quantity}value}validContract}}"
	QueryOutputs                = "{utxos(where: { address: { _eq: \"%s\"}}) {txHash index tokens {asset {policyId assetName} quantity} value}}"
	TransactionChaining         = &TransactionChainingMap{InputKey: UtxoKey{}, AssetsMap: make(map[string]string)}
	TransactionChainingKeyCache = &TransactionChainingKey{SpentUtxoMap: make(map[UtxoKey]bool), SpentUtxoListGropByTxHash: make(map[string]*[]UtxoKey)}
)

func AddTransactionChainingKeyCache(txhash string, txIns *[]UtxoKey) {
	for _, inputKey := range *txIns {
		TransactionChainingKeyCache.SpentUtxoMap[inputKey] = true
	}
	TransactionChainingKeyCache.SpentUtxoListGropByTxHash[txhash] = txIns
}

func ClearTransactionChainingKeyCache(txhash string) {
	if TransactionChainingKeyCache.SpentUtxoListGropByTxHash[txhash] != nil {
		list := TransactionChainingKeyCache.SpentUtxoListGropByTxHash[txhash]
		for _, key := range *list {
			delete(TransactionChainingKeyCache.SpentUtxoMap, key)
		}
		delete(TransactionChainingKeyCache.SpentUtxoListGropByTxHash, txhash)
	}
}

func ExecCmd(cmdStr, space string) (string, error) {
	if err := checkIllegal(cmdStr); err != nil {
		return "", err
	}
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

func checkIllegal(cmdName string) error {
	if strings.Contains(cmdName, "&") || strings.Contains(cmdName, "|") || strings.Contains(cmdName, ";") ||
		strings.Contains(cmdName, "$") || strings.Contains(cmdName, "'") || strings.Contains(cmdName, "`") ||
		strings.Contains(cmdName, "(") || strings.Contains(cmdName, ")") || strings.Contains(cmdName, "\"") {
		return tokens.ErrCmdArgVerify
	}
	return nil
}

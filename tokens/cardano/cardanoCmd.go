package cardano

import (
	"bytes"
	"math/big"
	"os/exec"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	AdaAsset  = "lovelace"
	TxTimeOut = 60 * 10
)

var (
	FixAdaAmount     = big.NewInt(1500000)
	DefaultAdaAmount = big.NewInt(2000000)
	QueryTransaction = "{transactions(where: { hash: { _eq: \"%s\"}}) {block {number epochNo slotNo}hash metadata{key value} inputs(order_by:{sourceTxHash:asc}){address value} outputs(order_by:{index:asc}){address index tokens{ asset{policyId assetName}quantity}value}validContract}}"
	QueryOutputs     = "{utxos(where: { address: { _eq: \"%s\"}}) {txHash index tokens {asset {policyId assetName} quantity} value}}"

	QueryTIPAndProtocolParams = "{ cardano { tip { number slotNo epoch { number protocolParams { coinsPerUtxoByte keyDeposit maxBlockBodySize maxBlockExMem maxTxSize maxValSize minFeeA minFeeB minPoolCost minUTxOValue} } } } }"

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

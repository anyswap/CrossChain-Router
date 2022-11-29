package reef

import (
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
)

type ReefTransaction struct {
	From         *string
	EvmAddress   *string
	To           *string
	Data         *hexutil.Bytes
	AccountNonce *uint64
	Amount       *big.Int
	GasLimit     *uint64
	StorageGas   *uint64
	BlockHash    *string
	BlockNumber  *uint64
	Signature    *string
	TxHash       *string
}

// const rawTx = args[0];
// const evmAddress = args[1];
// const substrateAddress = args[2];
// const toAddr = args[3];
// const totalLimit = BigNumber.from(args[4]);
// const storageLimit = BigNumber.from(args[5]);
// const blockHash = args[6];
// const blockNumber = BigNumber.from(args[7]);
// const nonce = BigNumber.from(args[8]);
func (tx *ReefTransaction) buildScriptParam() []interface{} {
	param := []interface{}{
		tx.Data,
		tx.EvmAddress,
		tx.From,
		tx.To,
		strconv.FormatUint(*tx.GasLimit, 10),
		strconv.FormatUint(*tx.StorageGas, 10),
		*tx.BlockHash,
		strconv.FormatUint(*tx.BlockNumber, 10),
		strconv.FormatUint(*tx.AccountNonce, 10),
	}
	if tx.Signature != nil {
		param = append(param, *tx.Signature)
	}

	return param
}

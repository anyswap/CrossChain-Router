package reef

import (
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

type ReefTransaction struct {
	From         *string
	EvmAddress   *string
	ReefAddress  *string
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
		*tx.EvmAddress,
		*tx.ReefAddress,
		*tx.To,
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

func buildRPCTxReceipt(tx string, extrinsic *Extrinsic, blockhash string, logs *[]EventLog, from *common.Address) (*types.RPCTxReceipt, error) {
	txHash := common.HexToHash(tx)
	var txIndex hexutil.Uint = hexutil.Uint(0)
	var blockNumber hexutil.Big = hexutil.Big(*new(big.Int).SetUint64(*extrinsic.BlockID))

	blockHash := common.HexToHash(blockhash)
	var status hexutil.Uint64
	if extrinsic.Status == "success" {
		status = hexutil.Uint64(1)
	} else {
		status = hexutil.Uint64(0)
	}

	to := common.HexToAddress(extrinsic.Args[0].String())

	fee, err := common.GetBigIntFromStr(extrinsic.SignedData.Fee.PartialFee)
	if err != nil {
		log.Warn("call GetBigIntFromStr error", "fee", extrinsic.SignedData.Fee.PartialFee, "err", err.Error())
		return nil, err
	}

	gasfee := hexutil.Uint64(fee.Uint64())

	rpclogs := []*types.RPCLog{}
	for _, log := range *logs {
		tlog := log.Data[0]
		address := common.HexToAddress(tlog.Address)

		topics := []common.Hash{}
		for _, topic := range tlog.Topics {
			topics = append(topics, common.HexToHash(topic))
		}
		rpclog := &types.RPCLog{
			Address: &address,
			Data:    tlog.Data,
			Topics:  topics,
		}
		rpclogs = append(rpclogs, rpclog)
	}

	result := &types.RPCTxReceipt{
		TxHash:      &txHash,
		TxIndex:     &txIndex,
		BlockNumber: &blockNumber,
		BlockHash:   &blockHash,
		Status:      &status,
		From:        from,
		Recipient:   &to,
		GasUsed:     &gasfee,
		Logs:        rpclogs,
	}
	return result, nil
}

func buildRPCTransaction(extrinsic *Extrinsic) *types.RPCTransaction {
	hash := common.HexToHash(*extrinsic.Hash)
	payload := extrinsic.Args[1]
	var number hexutil.Big = hexutil.Big(*new(big.Int).SetUint64(*extrinsic.BlockID))
	tx := &types.RPCTransaction{
		Hash:        &hash,
		Payload:     payload,
		BlockNumber: &number,
	}
	return tx
}

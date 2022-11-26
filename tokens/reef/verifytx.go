package reef

import (
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/common/hexutil"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

func (b *Bridge) GetTransactionReceipt(txHash string) (receipt *types.RPCTxReceipt, err error) {
	receipt, err = b.getTransactionReceipt(txHash)
	return receipt, err
}

func (b *Bridge) getTransactionReceipt(txHash string) (result *types.RPCTxReceipt, err error) {
	if len(b.WS) == 0 {
		return nil, errEmptyURLs
	}
	for _, ws := range b.WS {
		if ws.IsClose {
			continue
		}
		start := time.Now()
		extrinsic, err := ws.QueryTx(txHash)
		log.Info("call getTransactionReceipt finished", "txhash", txHash, "url", ws.endpoint, "timespent", time.Since(start).String(), "err", err != nil)
		if err != nil {
			log.Warn("call getTransactionReceipt error", "err", err.Error())
			continue
		}
		if extrinsic == nil {
			log.Warn("call getTransactionReceipt tx not found", "txhash", txHash)
			break
		}
		if extrinsic.BlockID == nil || extrinsic.ID == nil || extrinsic.Hash == nil {
			return nil, errTxReceiptMissBlockInfo
		}
		if !common.IsEqualIgnoreCase(*extrinsic.Hash, txHash) {
			return nil, errTxHashMismatch
		}
		if params.IsCheckTxBlockHashEnabled(b.ChainConfig.ChainID) {
			start = time.Now()
			errt := b.checkTxBlockHash(extrinsic.BlockID)
			log.Info("call checkTxBlockHash finished", "txhash", txHash, "block", extrinsic.BlockID, "timespent", time.Since(start).String())
			if errt != nil {
				return nil, errt
			}
		}

		logs, err := ws.QueryEventLogs(*extrinsic.ID)
		if err != nil {
			log.Warn("call QueryEventLogs error", "err", err.Error())
			continue
		}
		if logs == nil {
			log.Warn("call QueryEventLogs not found", "txhash", txHash)
			break
		}

		bh, err := b.GetGetBlockHash(*extrinsic.BlockID)
		if err != nil {
			log.Warn("call GetGetBlockHash error", "err", err.Error())
			break
		}
		from, err := ws.QueryEvmAddress(extrinsic.Signer)
		if err != nil {
			log.Warn("call QueryEvmAddress error", "err", err.Error())
			break
		}

		result, err := buildRPCTxReceipt(txHash, extrinsic, bh, logs, from)
		if err != nil {
			log.Warn("call GetGetBlockHash error", "err", err.Error())
			break
		}
		return result, nil
	}
	return nil, wrapRPCQueryError(err, "eth_getTransactionReceipt", txHash)
}

func buildRPCTxReceipt(tx string, extrinsic *Extrinsic, blockhash string, logs *[]EventLog, from *common.Address) (*types.RPCTxReceipt, error) {
	txHash := common.HexToHash(tx)
	var txIndex hexutil.Uint = hexutil.Uint(0)
	var blockNumber hexutil.Big = hexutil.Big(*new(big.Int).SetUint64(*extrinsic.BlockID))

	blockHash := common.HexToHash(blockhash)
	var status *hexutil.Uint64
	if extrinsic.Status == "success" {
		*status = hexutil.Uint64(1)
	} else {
		*status = hexutil.Uint64(0)
	}

	to := common.HexToAddress(extrinsic.Args[0])

	fee, err := common.GetBigIntFromStr(extrinsic.SignedData.Fee.PartialFee)
	if err != nil {
		log.Warn("call GetBigIntFromStr error", "fee", extrinsic.SignedData.Fee.PartialFee, "err", err.Error())
		return nil, err
	}

	var gasfee *hexutil.Uint64
	*gasfee = hexutil.Uint64(fee.Uint64())

	rpclogs := []*types.RPCLog{}
	for _, log := range *logs {
		tlog := log.Data[0]
		address := common.HexToAddress(tlog.Address)
		var data hexutil.Bytes = common.Hex2Bytes(tlog.Data)

		topics := []common.Hash{}
		for _, topic := range tlog.Topics {
			topics = append(topics, common.HexToHash(topic))
		}
		rpclog := &types.RPCLog{
			Address: &address,
			Data:    &data,
			Topics:  topics,
		}
		rpclogs = append(rpclogs, rpclog)
	}

	result := &types.RPCTxReceipt{
		TxHash:      &txHash,
		TxIndex:     &txIndex,
		BlockNumber: &blockNumber,
		BlockHash:   &blockHash,
		Status:      status,
		From:        from,
		Recipient:   &to,
		GasUsed:     gasfee,
		Logs:        rpclogs,
	}
	return result, nil
}

func (b *Bridge) checkTxBlockHash(blockNumber *uint64) error {
	block, err := b.GetLatestBlockNumber()
	if err != nil {
		return err
	}
	if block < *blockNumber {
		log.Warn("tx block hash mismatch", "LatestBlockNumber", block, "txBlockNumber", *blockNumber)
		return errTxBlockHashMismatch
	}
	return nil
}

// VerifyTransaction api
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType, tokens.ERC20SwapTypeMixPool:
		return b.VerifyERC20SwapTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

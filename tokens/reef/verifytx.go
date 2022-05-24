package reef

import (
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/types"
)

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(extKeyRaw string) (*tokens.TxStatus, error) {
	extRes, err := b.GetTransactionByHash(extKeyRaw)
	if err != nil {
		return nil, err
	}

	blockHash := ExtKeyFromRaw(extKeyRaw)

	blockRes, err := b.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}
	blockNumber, ok = new(big.Int).SetString(blockRes.Number, 0)
	if !ok {
		return nil, errors.New("block number error")
	}

	var txStatus tokens.TxStatus

	txStatus.Receipt = extRes
	txStatus.BlockHeight = blockNumber.Uint64()
	txStatus.BlockHash = blockHash

	if txStatus.BlockHeight != 0 {
		for i := 0; i < 3; i++ {
			latest, errt := b.GetLatestBlockNumber()
			if errt == nil {
				if latest > txStatus.BlockHeight {
					txStatus.Confirmations = latest - txStatus.BlockHeight
				}
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	return &txStatus, nil
}

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) error {
	tx, ok := rawTx.(*types.Transaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	msgHash := msgHashes[0]
	signer := b.Signer
	sigHash := signer.Hash(tx)
	if sigHash.String() != msgHash {
		log.Trace("message hash mismatch", "want", msgHash, "have", sigHash.String())
		return tokens.ErrMsgHashMismatch
	}
	return nil
}

// VerifyTransaction api
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifyERC20SwapTx(txHash, logIndex, allowUnstable)
	case tokens.NFTSwapType:
		return b.verifyNFTSwapTx(txHash, logIndex, allowUnstable)
	case tokens.AnyCallSwapType:
		return b.verifyAnyCallSwapTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

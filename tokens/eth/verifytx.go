package eth

import (
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (*tokens.TxStatus, error) {
	txr, err := b.GetTransactionReceipt(txHash)
	if err != nil {
		return nil, err
	}

	var txStatus tokens.TxStatus

	txStatus.Receipt = txr
	txStatus.BlockHeight = txr.BlockNumber.ToInt().Uint64()
	txStatus.BlockHash = txr.BlockHash.String()

	if txStatus.BlockHeight != 0 {
		for i := 0; i < 3; i++ {
			confirmations, errt := b.GetBlockConfirmations(txr)
			if errt == nil {
				txStatus.Confirmations = confirmations
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	return &txStatus, nil
}

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) error {
	if b.IsSapphireChain() {
		rawSapphire, ok := rawTx.(*SapphireRPCTx)
		log.Info("debug sapphire VerifyMsgHash 1111", "rawSapphire", rawSapphire, "ok", ok)
		if ok {
			tx2 := new(ethtypes.Transaction)
			err := tx2.UnmarshalBinary(rawSapphire.Raw)
			log.Info("debug sapphire VerifyMsgHash 2222", "tx2", tx2, "err", err)
			if err != nil {
				return tokens.ErrWrongRawTx
			}
			chainId := b.ChainConfig.GetChainID()
			signer := ethtypes.LatestSignerForChainID(chainId)
			msg, _ := tx2.AsMessage(signer, nil)
			log.Info("debug sapphire VerifyMsgHash 3333", "msg", msg, "msg.From().Hex()", msg.From().Hex(), "common.HexToAddress(rawSapphire.Sender).Hex()", common.HexToAddress(rawSapphire.Sender).Hex())
			if msg.From().Hex() == common.HexToAddress(rawSapphire.Sender).Hex() {
				log.Info("debug sapphire VerifyMsgHash 4444")
				return nil
			} else {
				log.Info("debug sapphire VerifyMsgHash 5555")
				return tokens.ErrWrongRawTx
			}
		}
	}
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
	case tokens.ERC20SwapType, tokens.ERC20SwapTypeMixPool:
		return b.verifyERC20SwapTx(txHash, logIndex, allowUnstable)
	case tokens.NFTSwapType:
		return b.verifyNFTSwapTx(txHash, logIndex, allowUnstable)
	case tokens.AnyCallSwapType:
		return b.verifyAnyCallSwapTx(txHash, logIndex, allowUnstable)
	case tokens.SapphireRPCType:
		return b.verifySapphireRPC(txHash, args)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

func (b *Bridge) verifySapphireRPC(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapTxInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapTxInfo.SwapType = tokens.SapphireRPCType
	chainID := b.ChainConfig.GetChainID()
	swapTxInfo.FromChainID = chainID
	swapTxInfo.ToChainID = chainID
	return swapTxInfo, nil
}

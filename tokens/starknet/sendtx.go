package starknet

import (
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet/rpcv02"
)

func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(rpcv02.BroadcastedInvokeV1Transaction)
	if !ok {
		return "", tokens.ErrWrongRawTx
	}
	output, err := b.provider.Invoke(tx)
	if err != nil {
		return "", err
	}
	return output.TransactionHash, nil
}

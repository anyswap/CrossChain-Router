package ripple

import (
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/websockets"
)

// SendTransaction send signed tx
func (b *Bridge) SendTransaction(signedTx interface{}) (txHash string, err error) {
	tx, ok := signedTx.(data.Transaction)
	if !ok {
		return "", tokens.ErrWrongRawTx
	}
	var success bool
	var resp *websockets.SubmitResult
	urls := append(b.GetGatewayConfig().APIAddress, b.GetGatewayConfig().APIAddressExt...)
	for i := 0; i < rpcRetryTimes; i++ {
		// try send to all remotes
		for _, url := range urls {
			r, err1 := websockets.NewRemote(url)
			if err1 != nil {
				log.Warn("Cannot connect to remote", "remote", url, "err", err1)
				continue
			}
			defer r.Close()

			resp, err = r.Submit(tx)
			if err != nil || resp == nil {
				log.Warn("Try sending transaction failed", "error", err)
				continue
			}
			if !resp.EngineResult.Success() {
				log.Warn("send tx with error result", "result", resp.EngineResult, "message", resp.EngineResultMessage)
			}
			txHash = tx.GetBase().Hash.String()
			success = true
		}
		if success {
			break
		}
		time.Sleep(rpcRetryInterval)
	}
	if success {
		if !params.IsParallelSwapEnabled() {
			b.SetNonce(tx.GetBase().Account.String(), uint64(tx.GetBase().Sequence)+1)
		}
		return txHash, nil
	}
	return "", err
}

// DoPostRequest only for test
func DoPostRequest(url, api, reqData string) string {
	apiAddress := url + "/" + api
	res, err := client.RPCRawPost(apiAddress, reqData)
	if err != nil {
		log.Warn("do post request failed", "url", apiAddress, "data", reqData, "err", err)
	}
	return res
}

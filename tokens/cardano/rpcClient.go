package cardano

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

const (
	queryTip    = "cardano-cli query tip --testnet-magic 1"
	rpcTimeout  = 60
	queryMethod = "{transactions(where: { hash: { _eq: \"%s\"}}) {block {number epochNo}hash metadata{key value}inputs{tokens{asset{ assetId assetName}quantity }value}outputs{address tokens{ asset{assetId assetName}quantity}value}validContract}}"
)

func GetTransactionMetadata(url string, msgID [32]byte) (interface{}, error) {
	return nil, tokens.ErrNotImplemented
}

func GetTransactionByHash(url, txHash string) (*Transaction, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(queryMethod, txHash)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result TransactionResult
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	} else {
		return &result.Transactions[0], nil
	}
}

func GetLatestBlockNumber(url string) (uint64, error) {
	if res, err := queryTipCmd(); err != nil {
		return 0, err
	} else {
		return res.Block, nil
	}
}

func queryTipCmd() (*Tip, error) {
	list := strings.Split(queryTip, " ")
	cmd := exec.Command(list[0], list[1:]...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		return nil, err
	} else {
		var tip Tip
		if err := json.Unmarshal(cmdOut.Bytes(), &tip); err != nil {
			return nil, err
		} else {
			return &tip, nil
		}
	}
}

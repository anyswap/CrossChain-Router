package main

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router/bridge"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosRouter"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cosmosSDK"
	"github.com/cosmos/cosmos-sdk/types"
)

var (
	paramMpc  = "cosmos1zc4qe220ceag38j0rwpanjfetp8lpkcvhutksa"
	BlockInfo = "/blocks/"
	urls      = []string{"https://cosmos-mainnet-rpc.allthatnode.com:1317", "https://v1.cosmos.network", "https://cosmoshub.stakesystems.io"}
)

func main() {
	log.SetLogger(6, false, true)

	cosmosRestClient := &cosmosSDK.CosmosRestClient{}
	cosmosRestClient.SetBaseUrls(urls)
	if res, err := GetBlockByNumber(12218750); err != nil {
		log.Fatal("GetBlockByNumber error", "err", err)
	} else {
		for _, tx := range res.Block.Data.Txs {
			if txBytes, err := base64.StdEncoding.DecodeString(tx); err == nil {
				txHash := fmt.Sprintf("%X", cosmosSDK.Sha256Sum(txBytes))
				if txRes, err := cosmosRestClient.GetTransactionByHash(txHash); err == nil {
					if err := ParseMemo(txRes.Tx.Body.Memo); err == nil {
						if err := ParseAmountTotal(txRes.TxResponse.Logs); err == nil {
							log.Info("verify txHash success", "txHash", txHash)
						}
					}
				}
			}
		}
	}
}

func ParseAmountTotal(messageLogs []types.ABCIMessageLog) (err error) {
	for _, logDetail := range messageLogs {
		for _, event := range logDetail.Events {
			if event.Type == cosmosRouter.TransferType {
				if (len(event.Attributes) == 2 || len(event.Attributes) == 3) && event.Attributes[0].Value == paramMpc {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("txHash not match")
}

func ParseMemo(memo string) error {
	fields := strings.Split(memo, ":")
	if len(fields) == 2 {
		if toChainID, err := common.GetBigIntFromStr(fields[1]); err != nil {
			return err
		} else {
			dstBridge := bridge.NewCrossChainBridge(toChainID)
			if dstBridge != nil && dstBridge.IsValidAddress(fields[0]) {
				return nil
			}
		}
	}
	return tokens.ErrTxWithWrongMemo
}

func GetBlockByNumber(blockNumber uint64) (*GetLatestBlockResponse, error) {
	var result *GetLatestBlockResponse
	for _, url := range urls {
		restApi := url + BlockInfo + fmt.Sprint(blockNumber)
		if err := client.RPCGet(&result, restApi); err == nil {
			return result, nil
		}
	}
	return nil, tokens.ErrRPCQueryError
}

type GetLatestBlockResponse struct {
	// Deprecated: please use `sdk_block` instead
	Block *Block `protobuf:"bytes,2,opt,name=block,proto3" json:"block,omitempty"`
}

type Block struct {
	Data Data `json:"data"`
}

type Data struct {
	Txs []string `json:"txs"`
}

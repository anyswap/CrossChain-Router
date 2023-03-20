package main

import (
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

const (
	url         = "https://graphql-api.mainnet.dandelion.link/"
	key         = "123"
	limit       = "10"
	mpcAddr     = ""
	QueryMethod = "{transactions(limit:%s order_by:{block:{slotNo:desc}}where:{metadata:{ key :{_eq:\"%s\"}}}){hash metadata{key value} outputs{address} validContract}}"

	TIP_QL = "{ cardano { tip { number slotNo epoch { number protocolParams { coinsPerUtxoByte keyDeposit maxBlockBodySize maxBlockExMem maxTxSize maxValSize minFeeA minFeeB minPoolCost minUTxOValue} } } } }"
)

func main() {
	log.SetLogger(6, false, true)

	var res []string
	if result, err := GetTransactionByMetadata(url, key); err != nil {
		log.Fatal("GetTransactionByMetadata fails", "err", err)
	} else {
		for _, transaction := range result.Transactions {
			for _, metadata := range transaction.Metadata {
				if metadata.Key == key {
					for _, utxo := range transaction.Outputs {
						if utxo.Address == mpcAddr {
							res = append(res, transaction.Hash)
						}
					}
				}
			}
		}
	}
	log.Info("fliter success", "res", res)

	tip, _ := QueryTip()
	log.Infof("tip %v", tip)

}

func GetTransactionByMetadata(url, key string) (*TransactionResult, error) {
	request := &client.Request{}
	request.Params = fmt.Sprintf(QueryMethod, limit, key)
	request.ID = int(time.Now().UnixNano())
	request.Timeout = 60
	var result TransactionResult
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type TransactionResult struct {
	Transactions []Transaction `json:"transactions"`
}

type Transaction struct {
	Hash          string     `json:"hash"`
	Metadata      []Metadata `json:"metadata"`
	Outputs       []Output   `json:"outputs"`
	ValidContract bool       `json:"validContract"`
}

type Output struct {
	Address string `json:"address"`
}

type Metadata struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func QueryTip() (*TipResponse, error) {
	request := &client.Request{}
	request.Params = TIP_QL
	request.ID = int(time.Now().UnixNano())
	request.Timeout = 60
	var result TipResponse
	if err := client.CardanoPostRequest(url, request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type TipResponse struct {
	Cardano NodeTip `json:"cardano"`
}

type NodeTip struct {
	Tip Tip `json:"tip"`
}

type Tip struct {
	BlockNumber uint64 `json:"number"`
	Epoch       Epoch  `json:"epoch"`
	SlotNo      uint64 `json:"slotNo"`
}

type Epoch struct {
	Number         uint64         `json:"number"`
	ProtocolParams ProtocolParams `json:"protocolParams"`
}

type ProtocolParams struct {
	CoinsPerUtxoByte uint64 `json:"coinsPerUtxoByte"`
	KeyDeposit       uint64 `json:"keyDeposit"`
	MaxBlockBodySize uint64 `json:"maxBlockBodySize"`
	MaxBlockExMem    string `json:"maxBlockExMem"`
	MaxTxSize        uint64 `json:"maxTxSize"`
	MaxValSize       string `json:"maxValSize"`
	MinFeeA          uint64 `json:"minFeeA"`
	MinFeeB          uint64 `json:"minFeeB"`
	MinPoolCost      uint64 `json:"minPoolCost"`
	MinUTxOValue     uint64 `json:"minUTxOValue"`
}

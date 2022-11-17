package main

import (
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

const (
	url         = "https://graphql-api.testnet.dandelion.link/"
	key         = "123"
	limit       = "10"
	mpcAddr     = ""
	QueryMethod = "{transactions(limit:%s order_by:{block:{slotNo:desc}}where:{metadata:{ key :{_eq:\"%s\"}}}){hash metadata{key value} outputs{address} validContract}}"
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

package main

import (
	"fmt"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

const (
	url = "https://graphql-api.mainnet.dandelion.link/"
	key = "123"
	// limit       = "20"
	mpcAddr = "addr1q88k2pae3l2n9kngjazxhj26yqht09645g8jdvyz4jfy0mxknshwg6haqqt9xmz2klhtgv89d5yzl2q9l0m5x4pt4ldqvrjmul"
	// QueryMethod = "{transactions(limit:%s order_by:{block:{slotNo:desc}}where:{metadata:{ key :{_eq:\"%s\"}}}){block{number slotNo} hash metadata{key value} outputs{address} validContract}}"

	QueryMethod2 = "{transactions(where:{block:{ number :{_eq: %d }} metadata:{ key :{_eq:\"%s\"}} }){block{number slotNo} hash metadata{key value} outputs{address} validContract}}"

	TIP_QL = "{ cardano { tip { number slotNo epoch { number protocolParams { coinsPerUtxoByte keyDeposit maxBlockBodySize maxBlockExMem maxTxSize maxValSize minFeeA minFeeB minPoolCost minUTxOValue} } } } }"
)

func main() {
	log.SetLogger(6, false, true)

	tip, _ := QueryTip()
	log.Infof("tip %d %d", tip.Cardano.Tip.BlockNumber, tip.Cardano.Tip.SlotNo)
	var res []string

	// query := fmt.Sprintf(QueryMethod, limit, key)
	query := fmt.Sprintf(QueryMethod2, tip.Cardano.Tip.BlockNumber, key)
	if result, err := GetTransactionByMetadata(url, query); err != nil {
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

func GetTransactionByMetadata(url, params string) (*TransactionResult, error) {
	request := &client.Request{}
	request.Params = params
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
	Block         Block      `json:"block"`
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

type Block struct {
	Number uint64 `json:"number"`
	SlotNo uint64 `json:"slotNo"`
}

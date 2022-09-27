package main

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"

	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

const (
	rpcTimeout = 60
	// target block
	startBlock = 94933767
	// token addr
	CONTRACT_ID = "t2.userdemo.testnet"
	// mpc addr
	MPC_ID = "userdemo.testnet"
	// get block info
	blockMethod = "block"
	// get chunk info
	chunkMethod = "chunk"
	// token transfer method
	ftTransferMethod = "ft_transfer"
	// anytoken swap out
	swapOutMethod = "swap_out"
	// near rpc url
	url = "https://archival-rpc.testnet.near.org"
)

func main() {
	blockDetails, err := GetBlockDetailsById(startBlock)
	if err != nil {
		log.Fatalf("GetBlockDetailsById err: '%v'", err)
	}
	chunksHash := FilterChunksHash(blockDetails)
	chunksDetail := FilterChunksDetail(chunksHash)
	if len(chunksDetail) == 0 {
		log.Fatalf("ChunksDetail len is zero")
	}
	transactionsHash := FilterTransactionsHash(chunksDetail)
	log.Info("FilterTransactionsHash", "transactionsHash", transactionsHash)
}

func GetBlockDetailsById(blockId uint) (*BlockDetail, error) {
	request := &client.Request{}
	request.Method = blockMethod
	request.Params = map[string]uint{"block_id": blockId}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result BlockDetail
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func GetChunkDetailsByHash(chunkHash string) (*ChunkDetail, error) {
	request := &client.Request{}
	request.Method = chunkMethod
	request.Params = map[string]string{"chunk_id": chunkHash}
	request.ID = int(time.Now().UnixNano())
	request.Timeout = rpcTimeout
	var result ChunkDetail
	err := client.RPCPostRequest(url, request, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func FilterChunksHash(blockDetails *BlockDetail) []string {
	var chunksHash []string
	for _, chunk := range blockDetails.Chunks {
		chunksHash = append(chunksHash, chunk.ChunkHash)
	}
	return chunksHash
}

func FilterChunksDetail(chunksHash []string) []*ChunkDetail {
	var chunksDetail []*ChunkDetail
	for _, chunkHash := range chunksHash {
		chunkDetail, err := GetChunkDetailsByHash(chunkHash)
		if err == nil {
			chunksDetail = append(chunksDetail, chunkDetail)
		}
	}
	return chunksDetail
}

func FilterTransactionsHash(chunksDetail []*ChunkDetail) []string {
	var transactionsHash []string
	for _, chunkDetail := range chunksDetail {
		transactions := chunkDetail.Transactions
		for _, transactionDetail := range transactions {
			if transactionDetail.Actions[0].FunctionCall.MethodName == ftTransferMethod && transactionDetail.ReceiverID == CONTRACT_ID {
				res, err := base64.StdEncoding.DecodeString(transactionDetail.Actions[0].FunctionCall.Args)
				if err != nil {
					log.Fatalf("Failed to decode base64: '%v'", err)
				}
				var ftTransferArgs FtTransfer
				err = json.Unmarshal(res, &ftTransferArgs)
				if err != nil {
					log.Fatalf("Failed to Unmarshal: '%v'", err)
				}
				if ftTransferArgs.ReceiverId == MPC_ID {
					transactionsHash = append(transactionsHash, transactionDetail.Hash)
				}
			} else if transactionDetail.Actions[0].FunctionCall.MethodName == swapOutMethod && (transactionDetail.ReceiverID == CONTRACT_ID || transactionDetail.ReceiverID == MPC_ID) {
				transactionsHash = append(transactionsHash, transactionDetail.Hash)
			}
		}
	}
	return transactionsHash
}

type BlockDetail struct {
	Chunks []Chunk `json:"chunks"`
}

type Chunk struct {
	ChunkHash string `json:"chunk_hash"`
}

type ChunkDetail struct {
	Transactions []TransactionDetail `json:"transactions"`
}

type TransactionDetail struct {
	Actions    []Action `json:"actions"`
	Hash       string   `json:"hash"`
	ReceiverID string   `json:"receiver_id"`
}

type Action struct {
	FunctionCall FunctionCall `json:"FunctionCall"`
}

type FunctionCall struct {
	Args       string `json:"args"`
	MethodName string `json:"method_name"`
}

type FtTransfer struct {
	ReceiverId string `json:"receiver_id"`
	Amount     string `json:"amount"`
	Memo       string `json:"memo"`
}

package main

import (
	"context"
	"flag"
	"fmt"
	"reflect"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/dontpanicdao/caigo/rpcv01"
	"github.com/dontpanicdao/caigo/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// var testnetRPC = "https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f"
// var mainnetRPC = "https://starknet-mainnet.infura.io/v3/435b852a9bcc4debb7b375a2727b296f"

var (
	paramURL         string
	paramBlockNumber uint64
	paramBlockHash   string
)

var ctx = context.Background()

func initFlags() {
	flag.StringVar(&paramURL, "network", "", "network, eg. https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f")
	flag.Uint64Var(&paramBlockNumber, "number", 0, "block number")
	flag.StringVar(&paramBlockHash, "hash", "", "block hash")
	flag.Parse()
}

func main() {
	log.SetLogger(6, false, true)
	initFlags()

	c, err := rpc.DialContext(ctx, paramURL)
	if err != nil {
		log.Fatalf("RPC dial failed: ", err)
	}
	p := rpcv01.NewProvider(c)

	if paramURL == "" {
		log.Fatalf("RPC endpoint is required")
	}

	var block *rpcv01.Block
	if paramBlockHash != "" {
		block = blockByHash(p)
	} else if paramBlockNumber != 0 {
		block = blockByNumber(p)
	} else {
		block = latestBlock(p)
	}
	printBlock(block)
}

func blockByNumber(p *rpcv01.Provider) *rpcv01.Block {
	fmt.Println("========== Get block by number ==========")
	blockInfo := rpcv01.BlockID{Number: &paramBlockNumber}
	rawBlock, err := p.BlockWithTxs(ctx, blockInfo)
	if err != nil {
		log.Fatalf("get block failed: ", err)
	}
	block, _ := validate(rawBlock)
	return block
}

func blockByHash(p *rpcv01.Provider) *rpcv01.Block {
	fmt.Println("========== Get block by hash ==========")
	h := types.HexToHash(paramBlockHash)
	blockInfo := rpcv01.BlockID{Hash: &h}
	rawBlock, err := p.BlockWithTxs(ctx, blockInfo)
	if err != nil {
		log.Fatalf("get block failed: ", err)
	}
	block, _ := validate(rawBlock)
	return block
}

func latestBlock(p *rpcv01.Provider) *rpcv01.Block {
	fmt.Println("========== Get latest block ==========")
	blockInfo := rpcv01.BlockID{Tag: "latest"}
	rawBlock, err := p.BlockWithTxs(ctx, blockInfo)
	if err != nil {
		log.Fatalf("get block failed: ", err)
	}
	block, _ := validate(rawBlock)
	return block
}

func validate(rawBlock interface{}) (*rpcv01.Block, bool) {
	if rawBlock == nil {
		log.Fatalf("nil block")
	}
	blockWithTxs, ok := rawBlock.(*rpcv01.Block)
	if !ok {
		log.Fatalf("expecting *rpv01.Block, instead %T", rawBlock)
	}

	if !strings.HasPrefix(blockWithTxs.BlockHash.String(), "0x") {
		log.Fatal("Block Hash should start with \"0x\", instead ", blockWithTxs.BlockHash)
	}

	if len(blockWithTxs.Transactions) == 0 {
		log.Fatal("the number of transactions should not be 0")
	}
	return blockWithTxs, true
}

func printBlock(block *rpcv01.Block) {
	fmt.Printf("block number: %d, block hash: %s\n", block.BlockNumber, block.BlockHash)
	for _, rawTx := range block.Transactions {
		fmt.Printf("tx type: %s, tx hash: %s\n", reflect.TypeOf(rawTx), rawTx.Hash())
		switch rawTx.(type) {
		case rpcv01.DeployAccountTxn:
		case rpcv01.L1HandlerTxn:
		case rpcv01.InvokeTxnV0:
			tx := rawTx.(rpcv01.InvokeTxnV0)
			fmt.Println("entry point selector: ", tx.EntryPointSelector)
		case rpcv01.InvokeTxnV1:
			tx := rawTx.(rpcv01.InvokeTxnV1)
			fmt.Println("sender address: ", tx.SenderAddress)
		case rpcv01.DeclareTxn:
			tx := rawTx.(rpcv01.DeclareTxn)
			fmt.Println("declare class hash: ", tx.ClassHash)
		case rpcv01.DeployTxn:
			tx := rawTx.(rpcv01.DeployTxn)
			fmt.Println("contract address: ", tx.ContractAddress)
		default:
			log.Warnf("unsupported tx type: %s\n", reflect.TypeOf(rawTx))
		}
	}
}

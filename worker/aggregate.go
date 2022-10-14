package worker

// import (
// 	"math/big"
// 	"time"

// 	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
// 	"github.com/anyswap/CrossChain-Router/v3/mongodb"
// 	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
// )

// const (
// 	mainnetNetWork = "mainnet"
// 	testnetNetWork = "testnet"
// 	devnetNetWork  = "devnet"
// )

// var (
// 	utxoPageLimit = 100
// 	aggInterval   = 7 * 24 * time.Hour
// 	targetChainId = big.NewInt(0)
// )

// // StartAggregateJob aggregate job
// func StartAggregateJob() {
// 	logWorker("replace", "start router swap replace job")
// 	if cardano.BridgeInstance == nil {
// 		return
// 	}
// 	mongodb.MgoWaitGroup.Add(1)
// 	go DoAggregateJob()
// }

// func DoAggregateJob() {
// 	defer mongodb.MgoWaitGroup.Done()
// 	if utils.IsCleanuping() {
// 		return
// 	}
// 	logWorker("aggregate", "start aggregate job")
// 	doAggregateJob()
// 	logWorker("aggregate", "finish aggregate job")
// 	time.Sleep(aggInterval)
// }

// func doAggregateJob() {
// 	if utils.IsCleanuping() {
// 		return
// 	}
// 	if txHash, err := cardano.BridgeInstance.AggregateTx(); err != nil {
// 		logWorkerError("aggregate", "aggregate tx err", err)
// 	} else {
// 		logWorker("aggregate", "aggregate tx success txHash:", txHash)
// 	}
// }

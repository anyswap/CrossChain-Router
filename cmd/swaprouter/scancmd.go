package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
	"github.com/fsn-dev/fsn-go-sdk/efsn/ethclient"
	"github.com/urfave/cli/v2"
)

var (
	routerContractFlag = &cli.StringFlag{
		Name:  "routerContract",
		Usage: "router contract address",
	}

	whitelistFlag = &cli.StringSliceFlag{
		Name:  "whitelist",
		Usage: "white list of contracts call into router contract",
	}

	scanswapCommand = &cli.Command{
		Action:    scanswap,
		Name:      "scanswap",
		Usage:     "scan router swap and post register",
		ArgsUsage: " ",
		Description: `
scan router swap and post register to swap server
`,
		Flags: []cli.Flag{
			routerContractFlag,
			whitelistFlag,
			utils.GatewayFlag,
			utils.ChainIDFlag,
			utils.SwapServerFlag,
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.StableHeightFlag,
			utils.JobsFlag,
		},
	}

	routerSwapScannedBlocks = &cachedSacnnedBlocks{
		capacity:  100,
		nextIndex: 0,
		hashes:    make([]string, 100),
	}
)

type routerSwapScanner struct {
	chainID        string
	routerContract string
	whitelist      []string
	gateway        string
	swapServer     string
	startHeight    uint64
	endHeight      uint64
	stableHeight   uint64
	jobCount       uint64

	client *ethclient.Client
	ctx    context.Context

	rpcInterval   time.Duration
	rpcRetryCount int
}

func scanswap(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	scanner := &routerSwapScanner{
		ctx:           context.Background(),
		rpcInterval:   3 * time.Second,
		rpcRetryCount: 3,
	}
	scanner.routerContract = ctx.String(routerContractFlag.Name)
	scanner.whitelist = ctx.StringSlice(whitelistFlag.Name)
	scanner.chainID = ctx.String(utils.ChainIDFlag.Name)
	scanner.gateway = ctx.String(utils.GatewayFlag.Name)
	scanner.swapServer = ctx.String(utils.SwapServerFlag.Name)
	scanner.startHeight = ctx.Uint64(utils.StartHeightFlag.Name)
	scanner.endHeight = ctx.Uint64(utils.EndHeightFlag.Name)
	scanner.stableHeight = ctx.Uint64(utils.StableHeightFlag.Name)
	scanner.jobCount = ctx.Uint64(utils.JobsFlag.Name)

	log.Info("get argument success",
		"routerContract", scanner.routerContract,
		"chainID", scanner.chainID,
		"gateway", scanner.gateway,
		"swapServer", scanner.swapServer,
		"start", scanner.startHeight,
		"end", scanner.endHeight,
		"stable", scanner.stableHeight,
		"jobs", scanner.jobCount,
		"whitelist", scanner.whitelist,
	)

	scanner.verifyOptions()
	scanner.init()
	scanner.run()
	return nil
}

func (scanner *routerSwapScanner) verifyOptions() {
	if !common.IsHexAddress(scanner.routerContract) {
		log.Fatalf("invalid router address '%v'", scanner.routerContract)
	}
	if scanner.gateway == "" {
		log.Fatal("must specify gateway address")
	}
	if scanner.chainID == "" {
		log.Fatal("must specify chainID")
	}
	if scanner.swapServer == "" {
		log.Fatal("must specify swap server address")
	}
	if scanner.endHeight != 0 && scanner.startHeight >= scanner.endHeight {
		log.Fatalf("wrong scan range [%v, %v)", scanner.startHeight, scanner.endHeight)
	}
	if scanner.jobCount == 0 {
		log.Fatal("zero jobs specified")
	}
}

func (scanner *routerSwapScanner) init() {
	ethcli, err := ethclient.Dial(scanner.gateway)
	if err != nil {
		log.Fatal("ethclient.Dail failed", "gateway", scanner.gateway, "err", err)
	}
	scanner.client = ethcli

	var version string
	for i := 0; i < scanner.rpcRetryCount; i++ {
		err = client.RPCPost(&version, scanner.swapServer, "swap.GetVersionInfo")
		if err == nil {
			log.Info("get server version succeed", "version", version)
			break
		}
		log.Warn("get server version failed", "swapServer", scanner.swapServer, "err", err)
		time.Sleep(scanner.rpcInterval)
	}
	if version == "" {
		log.Fatal("get server version failed", "swapServer", scanner.swapServer)
	}
}

func (scanner *routerSwapScanner) run() {
	start := scanner.startHeight
	wend := scanner.endHeight
	if wend == 0 {
		wend = scanner.loopGetLatestBlockNumber()
	}
	if start == 0 {
		start = wend
	}

	scanner.doScanRangeJob(start, wend)

	if scanner.endHeight == 0 {
		scanner.scanLoop(wend)
	}
}

func (scanner *routerSwapScanner) doScanRangeJob(start, end uint64) {
	if start >= end {
		return
	}
	jobs := scanner.jobCount
	count := end - start
	step := count / jobs
	if step == 0 {
		jobs = 1
		step = count
	}
	wg := new(sync.WaitGroup)
	for i := uint64(0); i < jobs; i++ {
		from := start + i*step
		to := start + (i+1)*step
		if i+1 == jobs {
			to = end
		}
		wg.Add(1)
		go scanner.scanRange(i+1, from, to, wg)
	}
	if scanner.endHeight != 0 {
		wg.Wait()
	}
}

func (scanner *routerSwapScanner) scanRange(job, from, to uint64, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info(fmt.Sprintf("[%v] start scan range", job), "from", from, "to", to)

	for h := from; h < to; h++ {
		scanner.scanBlock(job, h, false)
	}

	log.Info(fmt.Sprintf("[%v] scan range finish", job), "from", from, "to", to)
}

func (scanner *routerSwapScanner) scanLoop(from uint64) {
	stable := scanner.stableHeight
	log.Info("start scan loop", "from", from, "stable", stable)
	for {
		latest := scanner.loopGetLatestBlockNumber()
		for h := latest; h > from; h-- {
			scanner.scanBlock(0, h, true)
		}
		if from+stable < latest {
			from = latest - stable
		}
		time.Sleep(5 * time.Second)
	}
}

func (scanner *routerSwapScanner) loopGetLatestBlockNumber() uint64 {
	for {
		header, err := scanner.client.HeaderByNumber(scanner.ctx, nil)
		if err == nil {
			log.Info("get latest block number success", "height", header.Number)
			return header.Number.Uint64()
		}
		log.Warn("get latest block number failed", "err", err)
		time.Sleep(scanner.rpcInterval)
	}
}

func (scanner *routerSwapScanner) loopGetBlock(height uint64) *types.Block {
	blockNumber := new(big.Int).SetUint64(height)
	for {
		block, err := scanner.client.BlockByNumber(scanner.ctx, blockNumber)
		if err == nil {
			return block
		}
		log.Warn("get block failed", "height", height, "err", err)
		time.Sleep(scanner.rpcInterval)
	}
}

func (scanner *routerSwapScanner) scanBlock(job, height uint64, cache bool) {
	block := scanner.loopGetBlock(height)
	blockHash := block.Hash().String()
	if cache && routerSwapScannedBlocks.isScanned(blockHash) {
		return
	}
	log.Info(fmt.Sprintf("[%v] scan block %v", job, height), "hash", blockHash, "txs", len(block.Transactions()))
	for _, tx := range block.Transactions() {
		scanner.scanTransaction(tx)
	}
	if cache {
		routerSwapScannedBlocks.addBlock(blockHash)
	}
}

func (scanner *routerSwapScanner) checkTxToAddress(tx *types.Transaction) bool {
	if tx.To() == nil {
		return false
	}

	txtoAddress := tx.To().String()

	if strings.EqualFold(txtoAddress, scanner.routerContract) {
		return true
	}

	for _, whiteAddr := range scanner.whitelist {
		if strings.EqualFold(txtoAddress, whiteAddr) {
			return true
		}
	}

	return false
}

func (scanner *routerSwapScanner) scanTransaction(tx *types.Transaction) {
	if !scanner.checkTxToAddress(tx) {
		return
	}

	txHash := tx.Hash()
	var receipt *types.Receipt
	var err error
	for i := 0; i < 3; i++ {
		receipt, err = scanner.client.TransactionReceipt(scanner.ctx, txHash)
		if err == nil {
			break
		}
		time.Sleep(scanner.rpcInterval)
	}
	if err != nil || receipt == nil || receipt.Status != 1 {
		return
	}

	for i := 1; i < len(receipt.Logs); i++ {
		rlog := receipt.Logs[i]
		if rlog.Removed {
			continue
		}
		if !strings.EqualFold(rlog.Address.String(), scanner.routerContract) {
			continue
		}
		logTopic := rlog.Topics[0].Bytes()
		switch {
		case bytes.Equal(logTopic, eth.LogAnySwapOutTopic):
		case bytes.Equal(logTopic, eth.LogAnySwapTradeTokensForTokensTopic):
		case bytes.Equal(logTopic, eth.LogAnySwapTradeTokensForNativeTopic):
		default:
			continue
		}
		scanner.postSwap(scanner.chainID, txHash.String(), i)
	}
}

func (scanner *routerSwapScanner) postSwap(chainID, txid string, logIndex int) {
	subject := "post router swap register"
	rpcMethod := "swap.RegisterRouterSwap"
	log.Info(subject, "chainid", chainID, "txid", txid, "logindex", logIndex)

	var result interface{}
	args := map[string]string{
		"chainid":  chainID,
		"txid":     txid,
		"logindex": fmt.Sprintf("%d", logIndex),
	}
	err := client.RPCPost(&result, scanner.swapServer, rpcMethod, args)
	if err != nil {
		log.Warn(subject+" result", "chainid", chainID, "txid", txid, "logindex", logIndex, "result", result, "err", err)
	} else {
		log.Info(subject+" result", "chainid", chainID, "txid", txid, "logindex", logIndex, "result", result)
	}
}

type cachedSacnnedBlocks struct {
	capacity  int
	nextIndex int
	hashes    []string
}

func (cache *cachedSacnnedBlocks) addBlock(blockHash string) {
	cache.hashes[cache.nextIndex] = blockHash
	cache.nextIndex = (cache.nextIndex + 1) % cache.capacity
}

func (cache *cachedSacnnedBlocks) isScanned(blockHash string) bool {
	for _, b := range cache.hashes {
		if b == blockHash {
			return true
		}
	}
	return false
}

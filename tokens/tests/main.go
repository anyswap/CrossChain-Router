package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	rpcserver "github.com/anyswap/CrossChain-Router/v3/rpc/server"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tests/config"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tests/eth"
	"github.com/anyswap/CrossChain-Router/v3/tokens/tests/template"
	"github.com/urfave/cli/v2"
)

var (
	clientIdentifier = "testrouter"
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""
	// The app that holds all commands and flags.
	app = utils.NewApp(clientIdentifier, gitCommit, gitDate, "the testrouter command line interface")

	bridge tokens.IBridge
)

func initApp() {
	// Initialize the CLI app and start action
	app.Action = swaprouter
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2020-2022 The CrossChain-Router Authors"
	app.Flags = []cli.Flag{
		utils.ConfigFileFlag,
		utils.LogFileFlag,
		utils.LogRotationFlag,
		utils.LogMaxAgeFlag,
		utils.VerbosityFlag,
		utils.JSONFormatFlag,
		utils.ColorFormatFlag,
	}
}

func main() {
	initApp()
	if err := app.Run(os.Args); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func swaprouter(ctx *cli.Context) error {
	utils.SetLogger(ctx)

	configFile := utils.GetConfigFilePath(ctx)
	config.LoadTestConfig(configFile)
	initRouter()

	go startWork()
	rpcserver.StartTestServer(config.TestConfig.Port)

	utils.TopWaitGroup.Wait()
	return nil
}

func initRouter() {
	testCfg := config.TestConfig

	params.IsTestMode = true
	params.EnableSignWithPrivateKey()
	params.SetDebugMode(testCfg.IsDebugMode)

	switch testCfg.Module {
	case "eth":
		bridge = eth.NewCrossChainBridge()
	case "template":
		bridge = template.NewCrossChainBridge()
	default:
		log.Fatalf("unimplemented test module '%v'", testCfg.Module)
	}

	bridge.SetGatewayConfig(testCfg.Gateway)
	bridge.SetChainConfig(testCfg.Chain)
	bridge.SetTokenConfig(testCfg.Token.ContractAddress, testCfg.Token)
	bridge.InitAfterConfig()

	_ = params.SetExtraConfig(
		&params.ExtraConfig{
			AllowCallByContract: testCfg.AllowCallByContract,
		},
	)
	log.Info("init router finished", "AllowCallByContract", params.AllowCallByContract())

	router.AllChainIDs = testCfg.GetAllChainIDs()
	router.AllTokenIDs = []string{testCfg.Token.TokenID}
	router.SetRouterInfo(testCfg.Token.RouterContract, testCfg.SignerAddress, "", "")

	tokensMap := make(map[string]string)
	router.MultichainTokens[strings.ToLower(testCfg.Token.TokenID)] = tokensMap

	swapConfigs := make(map[string]map[string]*tokens.SwapConfig)
	swapConfigs[testCfg.Token.TokenID] = make(map[string]*tokens.SwapConfig)
	swapCfg := testCfg.GetSwapConfig()

	for _, chainID := range router.AllChainIDs {
		router.RouterBridges[chainID.String()] = bridge
		tokensMap[chainID.String()] = testCfg.Token.ContractAddress
		swapConfigs[testCfg.Token.TokenID][chainID.String()] = swapCfg
		params.SetSignerPrivateKey(chainID.String(), testCfg.SignWithPrivateKey)
	}
	router.PrintMultichainTokens()

	tokens.SetSwapConfigs(swapConfigs)
}

func startWork() {
	for {
		args := <-config.ChanIn
		err := process(args)
		if err != nil {
			config.ChanOut <- err.Error()
		} else {
			config.ChanOut <- "success"
		}
	}
}

func process(opts map[string]string) error {
	log.Info("start to process", "opts", opts)
	testCfg := config.TestConfig
	swapType := tokens.GetRouterSwapType()

	// parse arguments
	txid := opts["txid"]
	if txid == "" {
		return fmt.Errorf("error: empty txid")
	}
	logIndex, _ := common.GetIntFromStr(opts["logindex"])
	log.Info("parse arguments sucess", "txid", txid, "logIndex", logIndex)

	// register tx
	registerArgs := &tokens.RegisterArgs{
		SwapType: swapType,
		LogIndex: logIndex,
	}
	registerOK := false
	infos, errs := bridge.RegisterSwap(txid, registerArgs)
	for i, err := range errs {
		if err == nil && infos[i] != nil {
			registerOK = true
			logIndex = infos[i].LogIndex
			log.Info("found real log index", "logIndex", logIndex)
			break
		}
	}
	if !registerOK {
		return fmt.Errorf("register tx failed: %v", errs)
	}

	// verify tx
	verifyArgs := &tokens.VerifyArgs{
		SwapType:      swapType,
		LogIndex:      logIndex,
		AllowUnstable: false,
	}
	swapInfo, err := bridge.VerifyTransaction(txid, verifyArgs)
	if err != nil {
		return fmt.Errorf("verify tx failed: %w", err)
	}
	log.Info("verify tx success", "txid", txid)

	// build tx
	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			SwapInfo:    swapInfo.SwapInfo,
			Identifier:  testCfg.Identifier,
			SwapID:      txid,
			SwapType:    swapInfo.SwapType,
			Bind:        swapInfo.Bind,
			LogIndex:    swapInfo.LogIndex,
			FromChainID: swapInfo.FromChainID,
			ToChainID:   swapInfo.ToChainID,
		},
		From:        testCfg.SignerAddress,
		OriginFrom:  swapInfo.From,
		OriginTxTo:  swapInfo.TxTo,
		OriginValue: swapInfo.Value,
	}
	rawTx, err := bridge.BuildRawTransaction(args)
	if err != nil {
		return fmt.Errorf("build tx failed: %w", err)
	}
	log.Info("build tx success")

	// sign tx
	signedTx, txHash, err := bridge.MPCSignTransaction(rawTx, args)
	if err != nil {
		return fmt.Errorf("sign tx failed: %w", err)
	}
	log.Info("sign tx success", "hash", txHash)

	// send tx
	txHash, err = bridge.SendTransaction(signedTx)
	if err != nil {
		return fmt.Errorf("send tx failed: %w", err)
	}
	log.Info("send tx success", "hash", txHash)

	return nil
}

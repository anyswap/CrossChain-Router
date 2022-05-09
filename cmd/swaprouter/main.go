// Command swaprouter is main program to start swap router or its sub commands.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	rpcserver "github.com/anyswap/CrossChain-Router/v3/rpc/server"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/worker"
	"github.com/urfave/cli/v2"
)

var (
	clientIdentifier = "swaprouter"
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""
	// The app that holds all commands and flags.
	app = utils.NewApp(clientIdentifier, gitCommit, gitDate, "the swaprouter command line interface")
)

func initApp() {
	// Initialize the CLI app and start action
	app.Action = swaprouter
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2017-2020 The CrossChain-Router Authors"
	app.Commands = []*cli.Command{
		adminCommand,
		configCommand,
		toolsCommand,
		utils.LicenseCommand,
		utils.VersionCommand,
	}
	app.Flags = []cli.Flag{
		utils.DataDirFlag,
		utils.ConfigFileFlag,
		utils.RunServerFlag,
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
	if ctx.NArg() > 0 {
		return fmt.Errorf("invalid command: %q", ctx.Args().Get(0))
	}
	isServer := ctx.Bool(utils.RunServerFlag.Name)

	params.SetDataDir(utils.GetDataDir(ctx), isServer)
	configFile := utils.GetConfigFilePath(ctx)
	config := params.LoadRouterConfig(configFile, isServer, true)

	tokens.InitRouterSwapType(config.SwapType)

	if isServer {
		appName := params.GetIdentifier()
		dbConfig := config.Server.MongoDB
		mongodb.MongoServerInit(
			appName,
			dbConfig.DBURLs,
			dbConfig.DBName,
			dbConfig.UserName,
			dbConfig.Password,
		)
		worker.StartRouterSwapWork(true)
		time.Sleep(100 * time.Millisecond)
		rpcserver.StartAPIServer()
	} else {
		worker.StartRouterSwapWork(false)
	}

	utils.TopWaitGroup.Wait()
	return nil
}

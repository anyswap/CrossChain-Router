package main

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/urfave/cli/v2"
)

var (
	configCommand = &cli.Command{
		Name:  "config",
		Usage: "config router swap",
		Description: `
config router swap
`,
		Subcommands: []*cli.Command{
			{
				Name:   "getAllChainIDs",
				Usage:  "get all chainIDs",
				Action: getAllChainIDs,
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:   "getAllTokenIDs",
				Usage:  "get all tokenIDs",
				Action: getAllTokenIDs,
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "isChainIDExist",
				Usage:     "is chainID exist",
				Action:    isChainIDExist,
				ArgsUsage: "<chainID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "isTokenIDExist",
				Usage:     "is tokenID exist",
				Action:    isTokenIDExist,
				ArgsUsage: "<tokenID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:   "getAllChainConfig",
				Usage:  "get all chain config",
				Action: getAllChainConfig,
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getAllMultichainTokenConfig",
				Usage:     "get all multichain token config",
				Action:    getAllMultichainTokenConfig,
				ArgsUsage: "<tokenID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getChainConfig",
				Usage:     "get chain config",
				Action:    getChainConfig,
				ArgsUsage: "<chainID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getTokenConfig",
				Usage:     "get token config",
				Action:    getTokenConfig,
				ArgsUsage: "<tokenID> <chainID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getSwapConfigs",
				Usage:     "get swap configs by tokenID",
				Action:    getSwapConfigs,
				ArgsUsage: "<tokenID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getFeeConfigs",
				Usage:     "get fee configs by tokenID",
				Action:    getFeeConfigs,
				ArgsUsage: "<tokenID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getSwapConfig",
				Usage:     "get swap config by tokenID and source and dest chainID",
				Action:    getSwapConfig,
				ArgsUsage: "<tokenID> <fromChainID> <toChainID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getFeeConfig",
				Usage:     "get fee config by tokenID and source and dest chainID",
				Action:    getFeeConfig,
				ArgsUsage: "<tokenID> <fromChainID> <toChainID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getCustomConfig",
				Usage:     "get custom config",
				Action:    getCustomConfig,
				ArgsUsage: "<chainID> <key>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getExtraConfig",
				Usage:     "get extra config",
				Action:    getExtraConfig,
				ArgsUsage: "<key>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getAllMultichainTokens",
				Usage:     "get all multichain tokens",
				Action:    getAllMultichainTokens,
				ArgsUsage: "<tokenID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getMultichainToken",
				Usage:     "get multichain token",
				Action:    getMultichainToken,
				ArgsUsage: "<tokenID> <chainID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getMPCPubkey",
				Usage:     "get mpc address public key",
				Action:    getMPCPubkey,
				ArgsUsage: "<mpcAddress>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
		},
	}

	onchainContractFlag = &cli.StringFlag{
		Name:  "contract",
		Usage: "onchain contract address",
	}

	gatewaysFlag = &cli.StringSliceFlag{
		Name:  "gateway",
		Usage: "gateway URL to connect",
	}
)

func getAllChainConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	chainCfgs, err := router.GetAllChainConfig()
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(chainCfgs, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("chain configs are", string(jsdata))
	return nil
}

//nolint:dupl // allow duplicate
func getAllMultichainTokenConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	tokenCfgs, err := router.GetAllMultichainTokenConfig(tokenID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(tokenCfgs, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("swap configs are", string(jsdata))

	return nil
}

func getChainIDArgument(ctx *cli.Context, pos int) (chainID *big.Int, err error) {
	chainIDStr := ctx.Args().Get(pos)
	chainID, err = common.GetBigIntFromStr(chainIDStr)
	if err != nil {
		return nil, fmt.Errorf("wrong chainID '%v'", chainIDStr)
	}
	return chainID, nil
}

func getChainConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	chainID, err := getChainIDArgument(ctx, 0)
	if err != nil {
		return err
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	chainCfg, err := router.GetChainConfig(chainID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(chainCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("chain config is", string(jsdata))
	return nil
}

func getTokenConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 2 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	chainID, err := getChainIDArgument(ctx, 1)
	if err != nil {
		return err
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	tokenCfg, err := router.GetTokenConfig(chainID, tokenID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(tokenCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("token config is", string(jsdata))
	return nil
}

//nolint:dupl // allow duplicate
func getSwapConfigs(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	swapCfgs, err := router.GetSwapConfigs(tokenID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(swapCfgs, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("swap configs are", string(jsdata))

	return nil
}

//nolint:dupl // allow duplicate
func getFeeConfigs(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	swapCfgs, err := router.GetFeeConfigs(tokenID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(swapCfgs, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("swap configs are", string(jsdata))

	return nil
}

//nolint:dupl // allow duplicate
func getSwapConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 3 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	fromChainID, err := getChainIDArgument(ctx, 1)
	if err != nil {
		return err
	}
	toChainID, err := getChainIDArgument(ctx, 2)
	if err != nil {
		return err
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	swapCfg, err := router.GetSwapConfig(tokenID, fromChainID, toChainID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(swapCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("actual swap config is", string(jsdata))

	return nil
}

//nolint:dupl // allow duplicate
func getFeeConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 3 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	fromChainID, err := getChainIDArgument(ctx, 1)
	if err != nil {
		return err
	}
	toChainID, err := getChainIDArgument(ctx, 2)
	if err != nil {
		return err
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	feeCfg, err := router.GetFeeConfig(tokenID, fromChainID, toChainID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(feeCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("actual fee config is", string(jsdata))
	return nil
}

func getCustomConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 2 {
		return fmt.Errorf("miss required position argument")
	}
	chainID, err := getChainIDArgument(ctx, 0)
	if err != nil {
		return err
	}
	key := ctx.Args().Get(1)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	data, err := router.GetCustomConfig(chainID, key)
	if err != nil {
		return err
	}
	fmt.Println(data)
	return nil
}

func getExtraConfig(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	key := ctx.Args().Get(0)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	data, err := router.GetExtraConfig(key)
	if err != nil {
		return err
	}
	fmt.Println(data)
	return nil
}

func getMPCPubkey(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	mpcAddr := ctx.Args().Get(0)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	pubkey, err := router.GetMPCPubkey(mpcAddr)
	if err != nil {
		return err
	}
	fmt.Println(pubkey)
	return nil
}

//nolint:dupl // allow duplicate
func getAllMultichainTokens(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	tokenIDStr := ctx.Args().Get(0)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	mcTokens, err := router.GetAllMultichainTokens(tokenIDStr)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(mcTokens, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("token config is", string(jsdata))
	return nil
}

func getMultichainToken(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 2 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	chainID, err := getChainIDArgument(ctx, 1)
	if err != nil {
		return err
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	mcToken, err := router.GetMultichainToken(tokenID, chainID)
	if err != nil {
		return err
	}
	fmt.Println(mcToken)
	return nil
}

func getAllChainIDs(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	chainIDs, err := router.GetAllChainIDs()
	if err != nil {
		return err
	}
	fmt.Println(chainIDs)
	return nil
}

func getAllTokenIDs(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	tokenIDs, err := router.GetAllTokenIDs()
	if err != nil {
		return err
	}
	fmt.Println(tokenIDs)
	return nil
}

func isChainIDExist(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	chainID, err := getChainIDArgument(ctx, 0)
	if err != nil {
		return err
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	exist, err := router.IsChainIDExist(chainID)
	if err != nil {
		return err
	}
	fmt.Println(exist)
	return nil
}

func isTokenIDExist(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() < 1 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	if common.HasHexPrefix(tokenID) {
		tokenID = string(common.FromHex(tokenID))
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	exist, err := router.IsTokenIDExist(tokenID)
	if err != nil {
		return err
	}
	fmt.Println(exist)
	return nil
}

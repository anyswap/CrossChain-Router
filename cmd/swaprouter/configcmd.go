package main

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth/abicoder"
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
				Name:   "genSetChainConfigData",
				Usage:  "generate setChainConfig input data",
				Action: genSetChainConfigData,
				Flags: []cli.Flag{
					cChainIDFlag,
					cBlockChainFlag,
					cRouterContractFlag,
					cConfirmationsFlag,
					cInitialHeightFlag,
				},
				Description: `
generate ChainConfig json marshal data
`,
			},
			{
				Name:   "genSetTokenConfigData",
				Usage:  "generate setTokenConfig input data",
				Action: genSetTokenConfigData,
				Flags: []cli.Flag{
					swapTypeFlag,
					cChainIDFlag,
					cTokenIDFlag,
					cDecimalsFlag,
					cContractAddressFlag,
					cContractVersionFlag,
				},
				Description: `
generate TokenConfig json marshal data
`,
			},
			{
				Name:   "genSetSwapConfigData",
				Usage:  "generate setSwapConfig input data",
				Action: genSetSwapConfigData,
				Flags: []cli.Flag{
					cToChainIDFlag,
					cTokenIDFlag,
					cMaximumSwapFlag,
					cMinimumSwapFlag,
					cBigValueThresholdFlag,
					cSwapFeeRateFlag,
					cMaximumSwapFeeFlag,
					cMinimumSwapFeeFlag,
				},
				Description: `
generate SwapConfig json marshal data
`,
			},
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
				Name:      "getUserTokenConfig",
				Usage:     "get user token config",
				Action:    getUserTokenConfig,
				ArgsUsage: "<tokenID> <chainID>",
				Flags: []cli.Flag{
					onchainContractFlag,
					gatewaysFlag,
				},
			},
			{
				Name:      "getSwapConfig",
				Usage:     "get swap config by tokenID and dest chainID",
				Action:    getSwapConfig,
				ArgsUsage: "<tokenID> <toChainID>",
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

	swapTypeFlag = &cli.StringFlag{
		Name:  "swaptype",
		Usage: "swap type (eg. erc20swap, nftswap, etc.)",
	}

	// --------- chain config -------------------

	cChainIDFlag = &cli.StringFlag{
		Name:  "c.ChainID",
		Usage: "block chain ID (require)",
	}

	cBlockChainFlag = &cli.StringFlag{
		Name:  "c.BlockChain",
		Usage: "block chain name (require)",
	}

	cRouterContractFlag = &cli.StringFlag{
		Name:  "c.RouterContract",
		Usage: "swap router contract address (require)",
	}

	cConfirmationsFlag = &cli.Uint64Flag{
		Name:  "c.Confirmations",
		Usage: "chain stable confirmations (require)",
	}

	cInitialHeightFlag = &cli.Uint64Flag{
		Name:  "c.InitialHeight",
		Usage: "initial swap height",
	}

	// --------- token config -------------------

	cTokenIDFlag = &cli.StringFlag{
		Name:  "c.TokenID",
		Usage: "token identifier (require)",
	}

	cDecimalsFlag = &cli.IntFlag{
		Name:  "c.Decimals",
		Usage: "token decimals (require)",
		Value: 18,
	}

	cContractAddressFlag = &cli.StringFlag{
		Name:  "c.ContractAddress",
		Usage: "token contract address (require)",
	}

	cContractVersionFlag = &cli.Uint64Flag{
		Name:  "c.ContractVersion",
		Usage: "token version number (require)",
	}

	// --------- swap config -------------------

	cToChainIDFlag = &cli.StringFlag{
		Name:  "c.ToChainID",
		Usage: "dest chain ID (require)",
	}

	cMaximumSwapFlag = &cli.StringFlag{
		Name:  "c.MaximumSwap",
		Usage: "maximum swap value (require)",
	}

	cMinimumSwapFlag = &cli.StringFlag{
		Name:  "c.MinimumSwap",
		Usage: "minimum swap value (require)",
	}

	cBigValueThresholdFlag = &cli.StringFlag{
		Name:  "c.BigValueThreshold",
		Usage: "big swap value threshold (require)",
	}

	cSwapFeeRateFlag = &cli.Float64Flag{
		Name:  "c.SwapFeeRate",
		Usage: "swap fee rate (eg. 0.001)",
	}

	cMaximumSwapFeeFlag = &cli.StringFlag{
		Name:  "c.MaximumSwapFee",
		Usage: "maximum swap fee",
	}

	cMinimumSwapFeeFlag = &cli.StringFlag{
		Name:  "c.MinimumSwapFee",
		Usage: "minimum swap fee",
	}
)

func genSetChainConfigData(ctx *cli.Context) error {
	chainCfg := &tokens.ChainConfig{
		ChainID:        ctx.String(cChainIDFlag.Name),
		BlockChain:     ctx.String(cBlockChainFlag.Name),
		RouterContract: ctx.String(cRouterContractFlag.Name),
		Confirmations:  ctx.Uint64(cConfirmationsFlag.Name),
		InitialHeight:  ctx.Uint64(cInitialHeightFlag.Name),
	}
	err := chainCfg.CheckConfig()
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(chainCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("chain config struct is", string(jsdata))
	funcHash := common.FromHex("0x46bd32f5")
	configData := abicoder.PackData(
		chainCfg.BlockChain,
		common.HexToAddress(chainCfg.RouterContract),
		chainCfg.Confirmations,
		chainCfg.InitialHeight,
	)
	chainID, _ := new(big.Int).SetString(chainCfg.ChainID, 0)
	inputData := abicoder.PackDataWithFuncHash(funcHash, chainID)
	inputData = append(inputData, common.LeftPadBytes([]byte{0x40}, 32)...)
	inputData = append(inputData, configData...)
	fmt.Println("set chain config input data is", common.ToHex(inputData))
	return nil
}

func genSetTokenConfigData(ctx *cli.Context) error {
	chainIDStr := ctx.String(cChainIDFlag.Name)
	chainID, err := common.GetBigIntFromStr(chainIDStr)
	if err != nil {
		return fmt.Errorf("wrong chainID '%v'", chainIDStr)
	}
	decimalsVal := ctx.Int(cDecimalsFlag.Name)
	if decimalsVal < 0 || decimalsVal > 256 {
		return fmt.Errorf("wrong decimals '%v'", decimalsVal)
	}
	swapType := ctx.String(swapTypeFlag.Name)
	tokens.InitRouterSwapType(swapType)
	tokenID := ctx.String(cTokenIDFlag.Name)
	decimals := uint8(decimalsVal)
	tokenCfg := &tokens.TokenConfig{
		TokenID:         tokenID,
		Decimals:        decimals,
		ContractAddress: ctx.String(cContractAddressFlag.Name),
		ContractVersion: ctx.Uint64(cContractVersionFlag.Name),
	}
	err = tokenCfg.CheckConfig()
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(tokenCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("tokenID is", tokenID)
	fmt.Println("chainID is", chainID)
	fmt.Println("token config struct is", string(jsdata))
	funcHash := common.FromHex("0xba6e0d0f")
	inputData := abicoder.PackDataWithFuncHash(funcHash,
		tokenID,
		chainID,
		decimals,
		common.HexToAddress(tokenCfg.ContractAddress),
		tokenCfg.ContractVersion,
	)
	fmt.Println("set token config input data is", common.ToHex(inputData))
	return nil
}

func genSetSwapConfigData(ctx *cli.Context) error {
	chainIDStr := ctx.String(cToChainIDFlag.Name)
	chainID, err := common.GetBigIntFromStr(chainIDStr)
	if err != nil {
		return fmt.Errorf("wrong chainID '%v'", chainIDStr)
	}
	tokenID := ctx.String(cTokenIDFlag.Name)
	decimals := uint8(18)
	tokenCfg := &tokens.SwapConfig{
		MaximumSwap:           tokens.ToBits(ctx.String(cMaximumSwapFlag.Name), decimals),
		MinimumSwap:           tokens.ToBits(ctx.String(cMinimumSwapFlag.Name), decimals),
		BigValueThreshold:     tokens.ToBits(ctx.String(cBigValueThresholdFlag.Name), decimals),
		SwapFeeRatePerMillion: uint64(ctx.Float64(cSwapFeeRateFlag.Name) * 1000000),
		MaximumSwapFee:        tokens.ToBits(ctx.String(cMaximumSwapFeeFlag.Name), decimals),
		MinimumSwapFee:        tokens.ToBits(ctx.String(cMinimumSwapFeeFlag.Name), decimals),
	}
	err = tokenCfg.CheckConfig()
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(tokenCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("tokenID is", tokenID)
	fmt.Println("toChainID is", chainID)
	fmt.Println("swap config struct is", string(jsdata))
	funcHash := common.FromHex("0xca29ee96")
	inputData := abicoder.PackDataWithFuncHash(funcHash,
		tokenID,
		chainID,
		tokenCfg.MaximumSwap,
		tokenCfg.MinimumSwap,
		tokenCfg.BigValueThreshold,
		tokenCfg.SwapFeeRatePerMillion,
		tokenCfg.MaximumSwapFee,
		tokenCfg.MinimumSwapFee,
	)
	fmt.Println("set swap config input data is", common.ToHex(inputData))
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

func getTokenConfigImpl(ctx *cli.Context, isUserConfig bool) error {
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
	var tokenCfg *tokens.TokenConfig
	if isUserConfig {
		tokenCfg, err = router.GetUserTokenConfig(chainID, tokenID)
	} else {
		tokenCfg, err = router.GetTokenConfig(chainID, tokenID)
	}
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

func getTokenConfig(ctx *cli.Context) error {
	return getTokenConfigImpl(ctx, false)
}

func getUserTokenConfig(ctx *cli.Context) error {
	return getTokenConfigImpl(ctx, true)
}

func getSwapConfig(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		return fmt.Errorf("miss required position argument")
	}
	tokenID := ctx.Args().Get(0)
	toChainID, err := getChainIDArgument(ctx, 1)
	if err != nil {
		return err
	}
	router.InitRouterConfigClientsWithArgs(
		ctx.String(onchainContractFlag.Name),
		ctx.StringSlice(gatewaysFlag.Name),
	)
	swapCfg, err := router.GetSwapConfig(tokenID, toChainID)
	if err != nil {
		return err
	}
	jsdata, err := json.MarshalIndent(swapCfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("swap config is", string(jsdata))
	return nil
}

func getCustomConfig(ctx *cli.Context) error {
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

func getMPCPubkey(ctx *cli.Context) error {
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

func getAllMultichainTokens(ctx *cli.Context) error {
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

package main

import (
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/admin"
	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/urfave/cli/v2"
)

var (
	adminCommand = &cli.Command{
		Name:  "admin",
		Usage: "admin router swap",
		Flags: append(admin.CommonFlags, utils.CommonLogFlags...),
		Description: `
admin router swap

usage:

swaprouter admin --swapserver XXX --keystore XXX --password XXX <subcommand> [command options] [arguments...]
`,
		Subcommands: []*cli.Command{
			{
				Name:      "maintain",
				Usage:     "maintain router",
				Action:    maintain,
				ArgsUsage: "<action> [comma separated arguments]",
				Description: `
maintain router.

examples:

<pause|unpause> <chainID[,chainID]...>

<whitelist|unwhitelist> callbycontract,<chainID>,<address[,address]...>
<whitelist|unwhitelist> callbycontractcodehash,<chainID>,<codehash[,codehash]...>
<whitelist|unwhitelist> bigvalue,<tokenID>,<address[,address]...>

<blacklist|unblacklist> chainid,<chainID[,chainID]...>
<blacklist|unblacklist> tokenid,<tokenID[,tokenID]...>
<blacklist|unblacklist> account,<address[,address]...>
`,
			},
			{
				Name:   "passbigvalue",
				Usage:  "pass swap with big value",
				Action: passbigvalue,
				Flags:  swapKeyFlags,
				Description: `
pass swap with big value
`,
			},
			{
				Name:   "reswap",
				Usage:  "reswap failed swap",
				Action: reswap,
				Flags:  swapKeyFlags,
				Description: `
reswap failed swap
`,
			},
			{
				Name:   "replaceswap",
				Usage:  "replace pending swap",
				Action: replaceswap,
				Flags:  append(swapKeyFlags, utils.GasPriceFlag),
				Description: `
replace pending swap with same nonce and new gas price
`,
			},
			{
				Name:   "forbidswap",
				Usage:  "forbid specified swap",
				Action: forbidswap,
				Flags:  append(swapKeyFlags, utils.MemoFlag),
				Description: `
forbid specified swap
`,
			},
			{
				Name:   "passforbiddenswapout",
				Usage:  "pass forbidden swapout",
				Action: passforbiddenswapout,
				Flags:  swapKeyFlags,
				Description: `
pass forbidden swapout
`,
			},
		},
	}

	swapKeyFlags = []cli.Flag{
		utils.ChainIDFlag,
		utils.TxIDFlag,
		utils.LogIndexFlag,
	}
)

func maintain(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	if ctx.NArg() == 0 {
		return fmt.Errorf("maintain: no action is specified")
	}

	method := "maintain"
	err := admin.Prepare(ctx)
	if err != nil {
		return err
	}

	action := ctx.Args().Get(0)
	arguments := ""
	if ctx.NArg() > 1 {
		arguments = ctx.Args().Get(1)
	}

	log.Printf("%v: %v %v", method, action, arguments)

	params := []string{action, arguments}
	result, err := admin.SwapAdmin(method, params)

	log.Printf("result is '%v'", result)
	return err
}

func getKeys(ctx *cli.Context) (chainID, txid, logIndex string, err error) {
	chainID = ctx.String(utils.ChainIDFlag.Name)
	if _, err = common.GetBigIntFromStr(chainID); err != nil || chainID == "" {
		err = fmt.Errorf("wrong chain id '%v'", chainID)
		return
	}
	txid = ctx.String(utils.TxIDFlag.Name)
	if !common.IsHexHash(txid) {
		err = fmt.Errorf("wrong tx id '%v'", txid)
		return
	}
	logIndex = fmt.Sprintf("%d", ctx.Int(utils.LogIndexFlag.Name))
	return
}

func getGasPrice(ctx *cli.Context) (gasPrice string, err error) {
	gasPrice = ctx.String(utils.GasPriceFlag.Name)
	if _, err = common.GetBigIntFromStr(gasPrice); err != nil {
		err = fmt.Errorf("wrong gas price '%v'", gasPrice)
	}
	return
}

func passbigvalue(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	method := "passbigvalue"
	err := admin.Prepare(ctx)
	if err != nil {
		return err
	}
	chainID, txid, logIndex, err := getKeys(ctx)
	if err != nil {
		return err
	}

	log.Printf("%v: %v %v %v", method, chainID, txid, logIndex)

	params := []string{chainID, txid, logIndex}
	result, err := admin.SwapAdmin(method, params)

	log.Printf("result is '%v'", result)
	return err
}

func reswap(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	method := "reswap"
	err := admin.Prepare(ctx)
	if err != nil {
		return err
	}
	chainID, txid, logIndex, err := getKeys(ctx)
	if err != nil {
		return err
	}

	log.Printf("%v: %v %v %v", method, chainID, txid, logIndex)

	params := []string{chainID, txid, logIndex}
	result, err := admin.SwapAdmin(method, params)

	log.Printf("result is '%v'", result)
	return err
}

func replaceswap(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	method := "replaceswap"
	err := admin.Prepare(ctx)
	if err != nil {
		return err
	}
	chainID, txid, logIndex, err := getKeys(ctx)
	if err != nil {
		return err
	}
	gasPrice, err := getGasPrice(ctx)
	if err != nil {
		return err
	}

	log.Printf("%v: %v %v %v %v", method, chainID, txid, logIndex, gasPrice)

	params := []string{chainID, txid, logIndex, gasPrice}
	result, err := admin.SwapAdmin(method, params)

	log.Printf("result is '%v'", result)
	return err
}

func forbidswap(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	method := "forbidswap"
	err := admin.Prepare(ctx)
	if err != nil {
		return err
	}
	chainID, txid, logIndex, err := getKeys(ctx)
	if err != nil {
		return err
	}

	memo := ctx.String(utils.MemoFlag.Name)

	log.Printf("%v: %v %v %v (memo: %v)", method, chainID, txid, logIndex, memo)

	params := []string{chainID, txid, logIndex, memo}
	result, err := admin.SwapAdmin(method, params)

	log.Printf("result is '%v'", result)
	return err
}

func passforbiddenswapout(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	method := "passforbiddenswapout"
	err := admin.Prepare(ctx)
	if err != nil {
		return err
	}
	chainID, txid, logIndex, err := getKeys(ctx)
	if err != nil {
		return err
	}

	log.Printf("%v: %v %v %v", method, chainID, txid, logIndex)

	params := []string{chainID, txid, logIndex}
	result, err := admin.SwapAdmin(method, params)

	log.Printf("result is '%v'", result)
	return err
}

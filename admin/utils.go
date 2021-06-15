package admin

import (
	"errors"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
	"github.com/urfave/cli/v2"
)

// common flags
var (
	swapServer string

	CommonFlags = []cli.Flag{
		utils.SwapServerFlag,
		utils.KeystoreFileFlag,
		utils.PasswordFileFlag,
	}
)

// SwapAdmin rpc call server `swap.AdminCall`
func SwapAdmin(method string, params []string) (result interface{}, err error) {
	rawTx, err := Sign(method, params)
	if err != nil {
		return "", err
	}
	timeout := 300
	reqID := 1010
	err = client.RPCPostWithTimeoutAndID(&result, timeout, reqID, swapServer, "swap.AdminCall", rawTx)
	return result, err
}

func loadKeyStore(ctx *cli.Context) error {
	keyfile := ctx.String(utils.KeystoreFileFlag.Name)
	passfile := ctx.String(utils.PasswordFileFlag.Name)
	return LoadKeyStore(keyfile, passfile)
}

func initSwapServer(ctx *cli.Context) error {
	swapServer = ctx.String(utils.SwapServerFlag.Name)
	if swapServer == "" {
		return errors.New("must specify swapserver")
	}
	return nil
}

// Prepare load keystore and init server
func Prepare(ctx *cli.Context) (err error) {
	err = loadKeyStore(ctx)
	if err != nil {
		return err
	}

	err = initSwapServer(ctx)
	if err != nil {
		return err
	}

	return nil
}

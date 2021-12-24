package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/mr-tron/base58"
	"github.com/urfave/cli/v2"
)

var (
	toolsCommand = &cli.Command{
		Name:  "tools",
		Usage: "useful tools",
		Flags: utils.CommonLogFlags,
		Description: `
useful tools
`,
		Subcommands: []*cli.Command{
			{
				Name:      "base58",
				Usage:     "base58 encoding/decoding",
				Action:    base58Codec,
				ArgsUsage: "[message]",
				Flags:     []cli.Flag{messageFlag, isDecodeFlag, isHexFlag},
			},
			{
				Name:      "base64",
				Usage:     "base64 encoding/decoding",
				Action:    base64Codec,
				ArgsUsage: "[message]",
				Flags:     []cli.Flag{messageFlag, isDecodeFlag, isHexFlag, isURLFlag, isRawFlag},
			},
			{
				Name:      "keccak256",
				Usage:     "calc keccak256 hash",
				Action:    keccak256Hash,
				ArgsUsage: "[message]",
				Flags:     []cli.Flag{messageFlag, isHexFlag},
			},
			{
				Name:      "sha256",
				Usage:     "calc sha256 hash",
				Action:    sha256Hash,
				ArgsUsage: "[message]",
				Flags:     []cli.Flag{messageFlag, isHexFlag},
			},
		},
	}

	messageFlag = &cli.StringFlag{
		Name:    "message",
		Aliases: []string{"m"},
		Usage:   "message text",
	}

	isDecodeFlag = &cli.BoolFlag{
		Name:    "decode",
		Aliases: []string{"d"},
		Usage:   "decode data",
	}

	isHexFlag = &cli.BoolFlag{
		Name:  "hex",
		Usage: "from or to hex string",
	}

	isURLFlag = &cli.BoolFlag{
		Name:  "url",
		Usage: "use URL encoding",
	}

	isRawFlag = &cli.BoolFlag{
		Name:  "raw",
		Usage: "omits padding characters",
	}
)

func getMessage(ctx *cli.Context) (string, error) {
	if ctx.NArg() > 1 {
		return "", fmt.Errorf("has more than one position argument: %v", ctx.Args())
	}
	var message string
	if ctx.NArg() == 1 {
		message = ctx.Args().Get(0) // positional args first
	} else {
		message = ctx.String(messageFlag.Name)
	}
	fmt.Printf("the message is '%v'\n", message)
	return message, nil
}

func base58Codec(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	message, err := getMessage(ctx)
	if err != nil {
		return err
	}
	isDecode := ctx.Bool(isDecodeFlag.Name)
	isHex := ctx.Bool(isHexFlag.Name)
	if isDecode {
		data, err := base58.Decode(message)
		if err != nil {
			return fmt.Errorf("base58 decode error: '%v'", err)
		}
		if isHex {
			fmt.Printf("base58 decoding to hex is '%v'\n", common.ToHex(data))
		} else {
			fmt.Printf("base58 decoding to text is '%v'\n", string(data))
		}
	} else {
		if isHex {
			encStr := base58.Encode(common.FromHex(message))
			fmt.Printf("base58 encoding from hex is '%v'\n", encStr)
		} else {
			encStr := base58.Encode([]byte(message))
			fmt.Printf("base58 encoding from text is '%v'\n", encStr)
		}
	}
	return nil
}

func base64Codec(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	message, err := getMessage(ctx)
	if err != nil {
		return err
	}
	isDecode := ctx.Bool(isDecodeFlag.Name)
	isHex := ctx.Bool(isHexFlag.Name)
	isURL := ctx.Bool(isURLFlag.Name)
	isRaw := ctx.Bool(isRawFlag.Name)
	b64Encoding := base64.StdEncoding
	if isURL {
		if isRaw {
			b64Encoding = base64.RawURLEncoding
		} else {
			b64Encoding = base64.URLEncoding
		}
	} else if isRaw {
		b64Encoding = base64.RawStdEncoding
	}
	if isDecode {
		data, err := b64Encoding.DecodeString(message)
		if err != nil {
			return err
		}
		if isHex {
			fmt.Printf("base64 decoding to hex is '%v'\n", common.ToHex(data))
		} else {
			fmt.Printf("base64 decoding to text is '%v'\n", string(data))
		}
	} else {
		if isHex {
			encStr := b64Encoding.EncodeToString(common.FromHex(message))
			fmt.Printf("base64 encoding from hex is '%v'\n", encStr)
		} else {
			encStr := b64Encoding.EncodeToString([]byte(message))
			fmt.Printf("base64 encoding from text is '%v'\n", encStr)
		}
	}
	return nil
}

func keccak256Hash(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	message, err := getMessage(ctx)
	if err != nil {
		return err
	}
	isHex := ctx.Bool(isHexFlag.Name)
	if isHex {
		calcHash := common.Keccak256Hash(common.FromHex(message))
		fmt.Printf("calc keccak256 hash from hex is '%v'\n", calcHash.Hex())
	} else {
		calcHash := common.Keccak256Hash([]byte(message))
		fmt.Printf("calc keccak256 hash from text is '%v'\n", calcHash.Hex())
	}
	return nil
}

func sha256Hash(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	message, err := getMessage(ctx)
	if err != nil {
		return err
	}
	isHex := ctx.Bool(isHexFlag.Name)
	if isHex {
		calcHash := common.Hash(sha256.Sum256(common.FromHex(message)))
		fmt.Printf("calc sha256 hash from hex is '%v'\n", calcHash.Hex())
	} else {
		calcHash := common.Hash(sha256.Sum256([]byte(message)))
		fmt.Printf("calc sha256 hash from text is '%v'\n", calcHash.Hex())
	}
	return nil
}

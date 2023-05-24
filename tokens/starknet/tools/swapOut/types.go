package main

import (
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens/starknet/rpcv02"
)

var mainnet = "https://starknet-mainnet.infura.io/v3/435b852a9bcc4debb7b375a2727b296f"
var testnet1_1 = "https://starknet-goerli.infura.io/v3/435b852a9bcc4debb7b375a2727b296f"
var testnet1_2 = "https://starknet-goerli.cartridge.gg"
var testnet2 = "https://starknet-goerli2.infura.io/v3/435b852a9bcc4debb7b375a2727b296f"

var gateway = "https://alpha4.starknet.io"

var client *rpcv02.Provider
var Config *mpc.Config

// Default Account
//const DefaultAccountPrivateKey = "0x3f8e6af9f6e0942f9222a10b8c12d0512774fdd41e5c05c355966f88faf0855"
//const DefaultAccountAddress = "0x05eb5f19dffbe783fa29b4492061f1c7ed4e979bb9b3006d5853d127a5135685"

// MPC Eth Account
//const DefaultAccountPrivateKey = "mpc_priv_key"
//const DefaultAccountAddress = "0x06aed585647bb16e2444c32a4465c361c9b1928d04df6c2daa62a911630a683d"
//
//const ContractAddress = "0x057e7a21721bf20a9da88d01fa5d24d771c94cf55d8fd513ebe16bbfcc54cd8f" // alpha-goerli

var groupID = "8db4956ad1c2313010f416e31ede5594a4e9b5255b0c2eba95429531474a6f880eb295264000fdef4a9024e9ca8096502055ff5c062c470c6ddb20386f6d5905"

var neededOracles uint32 = 2
var totalOracles uint32 = 2

var keystore = "/Users/ccsoloist/multichain/FastMulThreshold-DSA/test/keystore/UTC--2018-10-11T01-26-58.462416324Z--3a1b3b81ed061581558a81f11d63e03129347437"
var password = "/Users/ccsoloist/multichain/FastMulThreshold-DSA/test/passwdfile/passwdfile1"
var rpcAddr = "http://127.0.0.1:5871"

var mpcPubkey = "04e8082be957ea371256cf8477927d87d77889ae00d9a905a84e65c5db699d351ba49bc3db59f8b95c052e62eb2c0a5627c08e4421cc6120240be0aa82c2695c2a"
var defaultNode = &params.MPCNodeConfig{
	SignGroups: []string{
		"8db4956ad1c2313010f416e31ede5594a4e9b5255b0c2eba95429531474a6f880eb295264000fdef4a9024e9ca8096502055ff5c062c470c6ddb20386f6d5905",
	},
	KeystoreFile: &keystore,
	PasswordFile: &password,
	RPCAddress:   &rpcAddr,
}

var mpcConfig = &params.MPCConfig{
	SignTypeEC256K1:            "EC256K1",
	APIPrefix:                  "smpc_",
	RPCTimeout:                 10,
	SignTimeout:                120,
	MinIntervalToAddSignGroup:  3600,
	VerifySignatureInAccept:    false,
	GetAcceptListLoopInterval:  5,
	GetAcceptListRetryInterval: 3,
	MaxAcceptSignTimeInterval:  600,
	PendingInvalidAccept:       false,
	GroupID:                    &groupID,
	NeededOracles:              &neededOracles,
	TotalOracles:               &totalOracles,
	Mode:                       0,
	Initiators:                 []string{"0x3A1b3B81Ed061581558a81F11d63E03129347437"},
	DefaultNode:                defaultNode,
}

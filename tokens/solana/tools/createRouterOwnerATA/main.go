package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana"
	routerprog "github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/token"
	solanatools "github.com/anyswap/CrossChain-Router/v3/tokens/solana/tools"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
	"github.com/mr-tron/base58"
)

var (
	bridge = solana.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string

	paramPublicKey string
	paramPriKey    string
	owner          string
	ownerAta       string
	tokenProgramID string
	routerContract string

	mpcConfig *mpc.Config
	chainID   = big.NewInt(0)
	payer     types.PublicKey
)

func main() {

	initAll()

	if paramPriKey != "" {
		payer = types.MustPrivateKeyFromBase58(paramPriKey).PublicKey()
	} else {
		payer = types.MustPublicKeyFromBase58(paramPublicKey)
	}
	payerAddr := payer.String()
	fmt.Printf("payer: %v\n", payerAddr)
	b1, _ := bridge.GetBalance(payerAddr)
	fmt.Printf("payer sol: %v\n", b1)

	tokenPubkey := types.MustPublicKeyFromBase58(tokenProgramID)
	fmt.Printf("token programid: %v\n", tokenPubkey.String())

	ownerPubkey := types.MustPublicKeyFromBase58(owner)
	fmt.Printf("owner address: %v\n", ownerPubkey.String())

	ownerAtaPublicKey, bump, err := types.PublicKeyFindProgramAddress([][]byte{ownerPubkey.ToSlice(), token.TokenProgramID.ToSlice(), tokenPubkey.ToSlice()}, types.ATAProgramID)
	if err != nil {
		log.Fatalf("PublicKeyFindProgramAddress error %v", err)
	}
	fmt.Printf("ownerAtaPublicKey bump:%v address:%v\n", uint8(bump), ownerAtaPublicKey)

	routerContractPubkey := types.MustPublicKeyFromBase58(routerContract)
	fmt.Printf("router address: %v\n", routerContractPubkey.String())

	createMintAccount := routerprog.NewCreateATAInstruction(payer, ownerPubkey, tokenPubkey, ownerAtaPublicKey)
	createMintAccount.RouterProgramID = routerContractPubkey
	instructions := []types.TransactionInstruction{createMintAccount}

	m, _ := createMintAccount.Data()
	fmt.Printf("instructions0 length %v %v \n", len(m), base58.Encode(m))

	resp, err := bridge.GetLatestBlockhash()
	if err != nil {
		log.Fatalf("GetLatestBlockhash error %v", err)
	}
	blockHash := resp.Value.Blockhash
	fmt.Printf("blockHash:  %v %v\n", resp.Value.LastValidBlockHeight, blockHash)

	tx, err := types.NewTransaction(instructions, blockHash, types.TransactionPayer(payer))
	if err != nil {
		log.Fatalf("NewTransaction error %v", err)
	}
	signer := &solanatools.Signer{
		PublicKey:  paramPublicKey,
		PrivateKey: paramPriKey,
	}

	signData, _ := tx.Message.Serialize()
	fmt.Printf("sign: %v %v\n", len(signData), base64.StdEncoding.EncodeToString(signData))

	for _, a := range tx.Message.AccountKeys {
		fmt.Println(a.String())
	}

	txHash := solanatools.SignAndSend(mpcConfig, bridge, []*solanatools.Signer{signer}, tx)

	fmt.Printf("tx send success: %v\n", txHash)

	var txm *types.TransactionWithMeta
	for i := 0; i < 10; i++ {
		txResult, _ := bridge.GetTransaction(txHash)
		if txResult != nil {
			txm, _ = txResult.(*types.TransactionWithMeta)
			break
		}
		fmt.Println("get tx status ...")
		time.Sleep(5 * time.Second)
	}
	fmt.Printf("tx comfired success at slot: %v BlockTime: %v status: %v\n", uint64(txm.Slot), txm.BlockTime, txm.Meta.IsStatusOk())

	result, err := bridge.GetAccountInfo(ownerAtaPublicKey.String(), "jsonParsed")
	if err != nil {
		log.Fatalf("GetAccountInfo error %v", err)
	}
	fmt.Println("ownerAta address:", ownerAtaPublicKey)
	fmt.Println("accountInfo", result.Value.Data)

}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&paramPublicKey, "pubkey", "", "signer public key")
	flag.StringVar(&paramPriKey, "priKey", "", "signer priKey key")
	flag.StringVar(&owner, "owner", "", "mint owner address")
	flag.StringVar(&ownerAta, "ownerAta", "", "owner's associated token address")
	flag.StringVar(&tokenProgramID, "token", "", "token address")
	flag.StringVar(&routerContract, "router", "", "router address")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

	log.Info("init flags finished")
}

func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	mpcConfig = mpc.InitConfig(config.MPC, true)
	log.Info("init config finished, IsFastMPC: %v", mpcConfig.IsFastMPC)
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

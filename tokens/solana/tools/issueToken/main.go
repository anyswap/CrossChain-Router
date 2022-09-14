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
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/system"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/token"
	solanatools "github.com/anyswap/CrossChain-Router/v3/tokens/solana/tools"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

var (
	bridge = solana.NewCrossChainBridge()

	paramConfigFile string
	paramChainID    string

	paramPublicKey string
	paramPriKey    string
	mintAuthority  string

	mpcConfig *mpc.Config
	chainID   = big.NewInt(0)
	payer     types.PublicKey
)

func main() {

	initAll()
	space := 82

	if paramPriKey != "" {
		payer = types.MustPrivateKeyFromBase58(paramPriKey).PublicKey()
	} else {
		payer = types.MustPublicKeyFromBase58(paramPublicKey)
	}
	payerAddr := payer.String()
	fmt.Printf("payer: %v\n", payerAddr)
	b1, _ := bridge.GetBalance(payerAddr)
	fmt.Printf("payer sol: %v\n", b1)

	mintPublicKey, mintPriKey, _ := types.NewRandomPrivateKey()
	fmt.Printf("minter PriKey: %v\n", mintPriKey.String())
	fmt.Printf("minter address: %v\n", mintPublicKey.String())
	// needn't send sol to minter
	// bridge.AirDrop(mintPublicKey.String(), 10000000000000)
	mintAuthPublicKey := types.MustPublicKeyFromBase58(mintAuthority)
	fmt.Printf("owner address: %v\n", mintAuthPublicKey.String())

	lamports, err := bridge.GetMinimumBalanceForRentExemption(uint64(space))
	if err != nil {
		log.Fatalf("GetMinimumBalanceForRentExemption error %v", err)
	}
	fmt.Printf("crate mint account space: %v lamports: %v\n", space, lamports)

	createMintAccount := system.NewCreateAccountInstruction(lamports, uint64(space), token.TokenProgramID, payer, mintPublicKey)
	initMintAccount := token.NewInitializeMintInstruction(9, mintPublicKey, mintAuthPublicKey, &mintAuthPublicKey, system.SysvarRentProgramID)

	instructions := []types.TransactionInstruction{createMintAccount, initMintAccount}

	resp, err := bridge.GetLatestBlockhash()
	if err != nil {
		log.Fatalf("GetLatestBlockhash error %v", err)
	}
	blockHash := resp.Value.Blockhash
	// blockHash = types.MustPublicKeyFromBase58("DQVWxKzTtA84shb4i4JXRFy7JPiohSZwaBZjrj9Hik6")
	fmt.Printf("blockHash:  %v %v\n", resp.Value.LastValidBlockHeight, blockHash)

	tx, err := types.NewTransaction(instructions, blockHash, types.TransactionPayer(payer))
	if err != nil {
		log.Fatalf("NewTransaction error %v", err)
	}
	signer := &solanatools.Signer{
		PublicKey:  paramPublicKey,
		PrivateKey: paramPriKey,
	}
	minter := &solanatools.Signer{
		PublicKey:  "",
		PrivateKey: mintPriKey.String(),
	}

	signData, _ := tx.Message.Serialize()
	fmt.Printf("sign: %v %v\n", len(signData), base64.StdEncoding.EncodeToString(signData))

	txHash := solanatools.SignAndSend(mpcConfig, bridge, []*solanatools.Signer{signer, minter}, tx)

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
	fmt.Printf("token programId: %v\n", mintPublicKey.String())

	ownerPDAPubkey, bump, err := types.PublicKeyFindProgramAddress([][]byte{mintAuthPublicKey.ToSlice(), token.TokenProgramID.ToSlice(), mintPublicKey.ToSlice()}, types.ATAProgramID)
	if err != nil {
		log.Fatalf("PublicKeyFindProgramAddress error %v", err)
	}
	fmt.Printf("ownerPDAPubkey bump:%v address:%v\n", uint8(bump), ownerPDAPubkey)
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")

	flag.StringVar(&paramPublicKey, "pubkey", "", "signer solana address")
	flag.StringVar(&paramPriKey, "priKey", "", "signer base58 priKey")
	flag.StringVar(&mintAuthority, "owner", "", "mint owner address")

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

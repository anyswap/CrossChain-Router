package solana

import (
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	routerprog "github.com/anyswap/CrossChain-Router/v3/tokens/solana/programs/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens/solana/types"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}

	routerPDASeeds = [][]byte{[]byte("Router")}
)

// Bridge solana bridge
type Bridge struct {
	*tokens.CrossChainBridgeBase
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		CrossChainBridgeBase: tokens.NewCrossChainBridgeBase(),
	}
}

// SupportChainID support chainID
func SupportChainID(chainID *big.Int) bool {
	chainIDNum := chainID.Uint64()
	return chainIDNum == 245022934 || // mainnet
		chainIDNum == 245022940 || // testnet
		chainIDNum == 245022926 // devnet
}

func (b *Bridge) checkTokenMinter(routerPDA string, tokenCfg *tokens.TokenConfig) (err error) {
	if !tokenCfg.IsStandardTokenVersion() {
		return nil
	}
	isMinter, err := b.IsMinter(tokenCfg.ContractAddress, routerPDA)
	if err != nil {
		return err
	}
	if !isMinter {
		return fmt.Errorf("%v is not minter", routerPDA)
	}
	return nil
}

// ####### NEW IMPLEMENT ###########################################
// SetGatewayConfig set gateway config
func (b *Bridge) SetGatewayConfig(gatewayCfg *tokens.GatewayConfig) {
	b.CrossChainBridgeBase.SetGatewayConfig(gatewayCfg)
}

// SetTokenConfig set token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	if tokenCfg.RouterContract == "" {
		tokenCfg.RouterContract = b.ChainConfig.RouterContract
	}

	if tokens.IsERC20Router() {
		if b.IsNative(tokenAddr) {
			if tokenCfg.Decimals != 9 {
				log.Fatal("token decimals mismatch", "tokenID", tokenCfg.TokenID, "chainID", b.ChainConfig.ChainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract")
			}
		} else {
			decimals, errt := b.GetTokenDecimals(tokenAddr)
			if errt != nil {
				log.Fatal("get token decimals failed tokenAddr:", tokenAddr, "err", errt)
			}
			if decimals != tokenCfg.Decimals {
				log.Fatal("token decimals mismatch", "tokenID", tokenCfg.TokenID, "chainID", b.ChainConfig.ChainID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
			}
			// if the token is anytoken or issue by multichain
			routerInfo := router.GetRouterInfo(tokenCfg.RouterContract, b.ChainConfig.ChainID)
			err := b.checkTokenMinter(routerInfo.RouterPDA, tokenCfg)
			if err != nil {
				log.Fatal("check token minter failed", "tokenID", tokenCfg.TokenID, "chainID", b.ChainConfig.ChainID, "tokenAddr", tokenAddr, "err", err)
			}
		}
	}

	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)
}

func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) (err error) {
	if routerContract == "" {
		return nil
	}
	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)

	routerContractPubkey, err := types.PublicKeyFromBase58(routerContract)
	if err != nil {
		log.Fatal("get router contract pubkey failed", "routerContract", routerContract, "err", err)
		return
	}
	routerPDAPubkey, bump, err := types.PublicKeyFindProgramAddress(routerPDASeeds, routerContractPubkey)
	if err != nil {
		log.Fatal("get router pda failed", "seeds", routerPDASeeds, "routerContract", routerContract, "err", err)
		return
	}
	routerPDA := routerPDAPubkey.String()

	routerAccount, err := b.GetRouterAccount(routerContract)
	if err != nil {
		log.Fatal("get router account failed", "url", b.GatewayConfig.APIAddress[0], "routerContract", routerContract, "err", err)
	}
	if routerAccount.Bump != bump {
		log.Fatal("get router account bump mismatch", "routerContract", routerContract, "have", routerAccount.Bump, "want", bump)
	}
	log.Info("get router account success", "routerContract", routerContract, "routerAccount", routerAccount)

	routerMPC := routerAccount.MPC.String()
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Fatal("get mpc public key failed", "mpc", routerMPC, "err", err)
	}
	if err = b.VerifyMPCPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Fatal("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
	}
	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC: routerMPC,
			RouterPDA: routerPDA,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)
	routerprog.InitRouterProgram(routerContractPubkey)

	log.Info(fmt.Sprintf("[%5v] init routerContract success", chainID),
		"routerContract", routerContract, "routerMPC", routerMPC, "routerPDA", routerPDA)
	return nil
}

// GetTxBlockInfo impl NonceSetter interface
func (b *Bridge) GetTxBlockInfo(txHash string) (blockHeight, blockTime uint64) {
	txStatus, err := b.GetTransactionStatus(txHash)
	if err != nil {
		return 0, 0
	}
	return txStatus.BlockHeight, txStatus.BlockTime
}

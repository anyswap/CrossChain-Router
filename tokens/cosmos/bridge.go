package cosmos

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/base"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
)

var (
	// ensure Bridge impl tokens.CrossChainBridge
	_ tokens.IBridge = &Bridge{}
	// ensure Bridge impl tokens.NonceSetter
	_ tokens.NonceSetter = &Bridge{}
)

// Bridge base bridge
type Bridge struct {
	*base.NonceSetterBase

	TxConfig cosmosClient.TxConfig

	Prefix string
	Denom  string
}

// NewCrossChainBridge new bridge
func NewCrossChainBridge() *Bridge {
	return &Bridge{
		NonceSetterBase: base.NewNonceSetterBase(),
		TxConfig:        BuildNewTxConfig(),
	}
}

func (b *Bridge) SetPrefixAndDenom(prefix, denom string) {
	b.Prefix = prefix
	b.Denom = denom
}

// InitAfterConfig init variables (ie. extra members) after loading config
func (b *Bridge) InitAfterConfig() {
}

// InitRouterInfo init router info
func (b *Bridge) InitRouterInfo(routerContract string) (err error) {

	if routerContract == "" {
		return nil
	}

	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)

	extra := strings.Split(b.ChainConfig.Extra, ":")
	if len(extra) != 2 {
		return fmt.Errorf("chainConfig extra error")
	} else {
		b.SetPrefixAndDenom(extra[0], extra[1])
	}

	routerMPC := b.GetRouterContract("")
	if routerMPC == "" {
		log.Warn("get router mpc address return an empty address", "routerContract", routerContract)
		return fmt.Errorf("empty router mpc address")
	}
	log.Info("get router mpc address success", "chainID", chainID, "routerContract", routerContract, "routerMPC", routerMPC)
	routerMPCPubkey, err := router.GetMPCPubkey(routerMPC)
	if err != nil {
		log.Warn("get mpc public key failed", "mpc", routerMPC, "err", err)
		return err
	}
	if err = b.VerifyPubKey(routerMPC, routerMPCPubkey); err != nil {
		log.Warn("verify mpc public key failed", "mpc", routerMPC, "mpcPubkey", routerMPCPubkey, "err", err)
		return err
	}
	router.SetRouterInfo(
		routerContract,
		chainID,
		&router.SwapRouterInfo{
			RouterMPC: routerMPC,
		},
	)
	router.SetMPCPublicKey(routerMPC, routerMPCPubkey)

	log.Info(fmt.Sprintf("[%5v] init router info success", chainID), "routerContract", routerContract, "routerMPC", routerMPC)
	if mongodb.HasClient() {
		nextSwapNonce, err := mongodb.FindNextSwapNonce(chainID, strings.ToLower(routerMPC))
		if err == nil {
			log.Info("init next swap nonce from db", "chainID", chainID, "mpc", routerMPC, "nonce", nextSwapNonce)
			b.InitSwapNonce(b, routerMPC, nextSwapNonce)
		}
	}

	return nil
}

// SetTokenConfig set and verify token config
func (b *Bridge) SetTokenConfig(tokenAddr string, tokenCfg *tokens.TokenConfig) {
	b.CrossChainBridgeBase.SetTokenConfig(tokenAddr, tokenCfg)

	if tokenCfg == nil || !tokens.IsERC20Router() {
		return
	}

	isReload := router.IsReloading
	logErrFunc := log.GetLogFuncOr(isReload, log.Error, log.Fatal)

	tokenID := tokenCfg.TokenID

	decimals, errt := b.GetTokenDecimals(tokenAddr)
	if errt != nil {
		logErrFunc("get token decimals failed", "tokenID", tokenID, "tokenAddr", tokenAddr, "err", errt)
		if isReload {
			return
		}
	}
	if decimals != tokenCfg.Decimals {
		logErrFunc("token decimals mismatch", "tokenID", tokenID, "tokenAddr", tokenAddr, "inconfig", tokenCfg.Decimals, "incontract", decimals)
		if isReload {
			return
		}
	}
}

// GetTokenDecimals query token decimals
func (b *Bridge) GetTokenDecimals(tokenAddr string) (uint8, error) {
	return 6, nil
}

// GetTransaction impl
func (b *Bridge) GetTransaction(txHash string) (tx interface{}, err error) {
	return b.GetTransactionByHash(txHash)
}

// GetTransactionStatus impl
func (b *Bridge) GetTransactionStatus(txHash string) (status *tokens.TxStatus, err error) {
	status = new(tokens.TxStatus)
	if res, err := b.GetTransactionByHash(txHash); err != nil {
		log.Trace(b.ChainConfig.BlockChain+" GetTransactionStatus fail", "tx", txHash, "err", err)
		return status, err
	} else {
		if res.TxResponse.Code != 0 {
			return status, tokens.ErrTxWithWrongStatus
		}
		if txHeight, err := strconv.ParseUint(res.TxResponse.Height, 10, 64); err != nil {
			return status, err
		} else {
			status.BlockHeight = txHeight
		}
		if blockNumber, err := b.GetLatestBlockNumber(); err == nil {
			if blockNumber > status.BlockHeight {
				status.Confirmations = blockNumber - status.BlockHeight
			}
		}
	}
	return status, nil
}

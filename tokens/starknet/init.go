package starknet

import (
	"fmt"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/eth"
)

var (
	retryRPCCount    = 3
	retryRPCInterval = 1 * time.Second
)

func (b *Bridge) InitAfterConfig() {
	extra := strings.Split(b.ChainConfig.Extra, ":")
	if len(extra) != 2 {
		log.Warn("get router mpc address and account address failed ", extra)
	}

	address := extra[1]
	account, err := NewAccount(address, b.ChainID.String())
	if err != nil {
		log.Error("error creating starknet mpc account: ", err)
	}
	b.account = account
	//b.account.chainId = b.ChainID
	if b.provider == nil {
		for _, url := range b.GatewayConfig.AllGatewayURLs {
			provider, err := NewProvider(url, b.ChainID)
			if err == nil {
				b.provider = provider
				return
			}
			log.Error("error connecting to starknet rpc: ", err)
		}
	}
}

func (b *Bridge) InitRouterInfo(routerContract, routerVersion string) error {
	if routerContract == "" {
		return nil
	}

	chainID := b.ChainConfig.ChainID
	log.Info(fmt.Sprintf("[%5v] start init router info", chainID), "routerContract", routerContract)

	extra := strings.Split(b.ChainConfig.Extra, ":")
	if len(extra) != 2 {
		return fmt.Errorf("chainConfig extra error")
	}

	routerMPC := extra[0]
	accountAddress := extra[1]

	if routerMPC == "" {
		log.Warn("get router mpc address return an empty address", "routerContract", routerContract)
		return fmt.Errorf("empty router mpc address")
	}
	log.Info("get router mpc address success", "routerContract", routerContract, "routerMPC", routerMPC)
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
			b.InitSwapNonce(b, accountAddress, nextSwapNonce)
		}
	}

	return nil
}

func (b *Bridge) InitSwapNonce(br tokens.NonceSetter, address string, nonce uint64) {
	extra := strings.Split(b.ChainConfig.Extra, ":")
	if len(extra) != 2 {
		log.Warn("parse chain config extra failed init swap ", extra)
		return
	}
	accountAddress := extra[1]

	account := strings.ToLower(address)
	swapNonceLock := b.GetSwapNonceLock(account)
	swapNonceLock.RLock()
	defer swapNonceLock.RUnlock()

	dbNexNonce := nonce
	for i := 0; i < retryRPCCount; i++ {
		pendingNonce, err := br.GetPoolNonce(accountAddress, "pending")
		if err == nil {
			if pendingNonce > nonce {
				log.Warn("init swap nonce with onchain account nonce", "chainID", b.ChainConfig.ChainID, "dbNonce", nonce, "accountNonce", pendingNonce)
				nonce = pendingNonce
			}
			break
		}
		if i+1 == retryRPCCount {
			log.Warn("init swap nonce get account nonce failed", "chainID", b.ChainConfig.ChainID, "account", address, "err", err)
		}
		time.Sleep(retryRPCInterval)
	}
	b.SetNonce(address, nonce)
	log.Info("init swap nonce success", "chainID", b.ChainConfig.ChainID, "account", address, "dbNexNonce", dbNexNonce, "nonce", nonce)
}

func (b *Bridge) VerifyPubKey(address, pubKey string) error {
	pk, err := eth.PublicKeyToAddress(pubKey)
	if err != nil {
		return err
	}
	address = strings.ToLower(address)
	pk = strings.ToLower(pk)
	if !strings.EqualFold(address, pk) {
		return tokens.ErrMpcAddrMissMatch
	}
	return nil
}

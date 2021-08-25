// Package router inits bridges and loads onchain configs.
package router

import (
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// router bridges
var (
	RouterBridges    = make(map[string]tokens.IBridge)    // key is chainID
	MultichainTokens = make(map[string]map[string]string) // key is tokenID,chainID
	AllChainIDs      []*big.Int                           // all chainIDs is retrieved only once
	AllTokenIDs      []string                             // all tokenIDs can be reload
)

// GetBridgeByChainID get bridge by chain id
func GetBridgeByChainID(chainID string) tokens.IBridge {
	return RouterBridges[chainID]
}

// GetCachedMultichainTokens get multichain tokens of `tokenid`
func GetCachedMultichainTokens(tokenID string) map[string]string {
	tokenIDKey := strings.ToLower(tokenID)
	return MultichainTokens[tokenIDKey]
}

// GetCachedMultichainToken get multichain token address by tokenid and chainid
func GetCachedMultichainToken(tokenID, chainID string) (tokenAddr string) {
	tokenIDKey := strings.ToLower(tokenID)
	mcTokens := MultichainTokens[tokenIDKey]
	if mcTokens == nil {
		return ""
	}
	return mcTokens[chainID]
}

// PrintMultichainTokens print
func PrintMultichainTokens() {
	log.Info("*** begin print all multichain tokens")
	for tokenID, tokensMap := range MultichainTokens {
		log.Infof("*** multichain tokens of tokenID '%v' count is %v", tokenID, len(tokensMap))
		for chainID, tokenAddr := range tokensMap {
			log.Infof("*** multichain token: chainID %v tokenAddr %v", chainID, tokenAddr)
		}
	}
	log.Info("*** end print all multichain tokens")
}

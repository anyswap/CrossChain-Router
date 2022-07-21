package router

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// custom keys
const (
	additionalSrcChainSwapFeeRateKey = "additionalSrcChainSwapFeeRate"
)

func getCustomConfigKey(tokenID, keyID string) string {
	return fmt.Sprintf("%s/%s", tokenID, keyID)
}

// InitOnchainCustomConfig init onchain custom config
func InitOnchainCustomConfig(chainID *big.Int, tokenID string) {
	logErrFunc := log.GetLogFuncOr(DontPanicInLoading(), log.Error, log.Fatal)

	key := getCustomConfigKey(tokenID, additionalSrcChainSwapFeeRateKey)
	addtionalFeeRateStr, err := GetCustomConfig(chainID, key)
	if err != nil {
		logErrFunc("get custom config failed", "chainID", chainID, "key", key, "err", err)
		return
	}

	if addtionalFeeRateStr == "" {
		return
	}

	addtionalFeeRate, err := strconv.ParseFloat(addtionalFeeRateStr, 8)
	if err != nil {
		logErrFunc("wrong custom addtional fee rate", "chainID", chainID, "tokenID", tokenID, "key", key, "value", addtionalFeeRateStr, "err", err)
		return
	}

	ccConfig := tokens.GetOnchainCustomConfig(chainID.String(), tokenID)
	if ccConfig == nil {
		ccConfig = &tokens.OnchainCustomConfig{}
	}

	ccConfig.AdditionalSrcChainSwapFeeRate = addtionalFeeRate

	tokens.SetOnchainCustomConfig(chainID.String(), tokenID, ccConfig)
	log.Info("set onchain custom config", "chainID", chainID, "tokenID", tokenID, "config", ccConfig)
}

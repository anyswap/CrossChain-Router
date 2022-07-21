package router

import (
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// custom keys
const (
	additionalSrcChainSwapFeeRateKey = "additionalSrcChainSwapFeeRate"
)

// InitOnchainCustomConfig init onchain custom config
func InitOnchainCustomConfig(chainID *big.Int) {
	logErrFunc := log.GetLogFuncOr(DontPanicInLoading(), log.Error, log.Fatal)

	ccConfig := tokens.GetOnchainCustomConfig(chainID.String())
	if ccConfig == nil {
		ccConfig = &tokens.OnchainCustomConfig{}
	}

	key := additionalSrcChainSwapFeeRateKey
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
		logErrFunc("wrong custom addtional fee rate", "chainID", chainID, "key", key, "value", addtionalFeeRateStr, "err", err)
		return
	}
	ccConfig.AdditionalSrcChainSwapFeeRate = addtionalFeeRate

	tokens.SetOnchainCustomConfig(chainID.String(), ccConfig)
	log.Info("set onchain custom config", "chainID", chainID, "config", ccConfig)
}

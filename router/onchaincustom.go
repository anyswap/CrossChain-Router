package router

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

// custom keys
const (
	// value is `ratePerMillion,minFee,maxFee` with fee decimals of 18
	additionalSrcChainSwapFeeRateKey = "additionalSrcChainSwapFeeRate"
)

func getCustomConfigKey(tokenID, keyID string) string {
	return fmt.Sprintf("%s/%s", tokenID, keyID)
}

// InitOnchainCustomConfig init onchain custom config
func InitOnchainCustomConfig(chainID *big.Int, tokenID string) {
	logErrFunc := log.GetLogFuncOr(DontPanicInLoading(), log.Error, log.Fatal)

	key := getCustomConfigKey(tokenID, additionalSrcChainSwapFeeRateKey)
	addtionalFeeParams, err := GetCustomConfig(chainID, key)
	if err != nil {
		logErrFunc("get custom config failed", "chainID", chainID, "key", key, "err", err)
		return
	}

	if addtionalFeeParams == "" {
		return
	}

	addtionalFeeParts := strings.Split(addtionalFeeParams, ",")

	addtionalFeeRate, err := common.GetUint64FromStr(strings.TrimSpace(addtionalFeeParts[0]))
	if err != nil {
		logErrFunc("wrong custom addtional fee rate per million", "chainID", chainID, "tokenID", tokenID, "key", key, "value", addtionalFeeParams, "err", err)
		return
	}

	additionalMinFee := big.NewInt(0)
	additionalMaxFee := big.NewInt(0)

	switch len(addtionalFeeParts) {
	case 1:
	case 3:
		additionalMinFee, err = common.GetBigIntFromStr(strings.TrimSpace(addtionalFeeParts[1]))
		if err != nil {
			logErrFunc("wrong custom addtional min fee", "chainID", chainID, "tokenID", tokenID, "key", key, "value", addtionalFeeParams, "err", err)
			return
		}
		additionalMaxFee, err = common.GetBigIntFromStr(strings.TrimSpace(addtionalFeeParts[2]))
		if err != nil {
			logErrFunc("wrong custom addtional max fee", "chainID", chainID, "tokenID", tokenID, "key", key, "value", addtionalFeeParams, "err", err)
			return
		}
	default:
		logErrFunc("wrong additional fee params", "chainID", chainID, "key", key, "param", addtionalFeeParams)
		return
	}

	ccConfig := tokens.GetOnchainCustomConfig(chainID.String(), tokenID)
	if ccConfig == nil {
		ccConfig = &tokens.OnchainCustomConfig{}
	}

	ccConfig.AdditionalSrcChainSwapFeeRate = addtionalFeeRate
	ccConfig.AdditionalSrcMinimumSwapFee = additionalMinFee
	ccConfig.AdditionalSrcMaximumSwapFee = additionalMaxFee

	tokens.SetOnchainCustomConfig(chainID.String(), tokenID, ccConfig)
	log.Info("set onchain custom config", "chainID", chainID, "tokenID", tokenID, "config", ccConfig)
}

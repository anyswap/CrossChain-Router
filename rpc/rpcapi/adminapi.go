package rpcapi

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/admin"
	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/worker"
)

const (
	maintainCmd             = "maintain"
	passbigvalueCmd         = "passbigvalue"
	reswapCmd               = "reswap"
	replaceswapCmd          = "replaceswap"
	forbidSwapCmd           = "forbidswap"
	passForbiddenSwapoutCmd = "passforbiddenswapout"

	// maintain actions
	actPause       = "pause"
	actUnpause     = "unpause"
	actWhitelist   = "whitelist"
	actUnwhitelist = "unwhitelist"
	actBlacklist   = "blacklist"
	actUnblacklist = "unblacklist"

	successReuslt = "Success"
)

// AdminCall admin call
func (s *RouterSwapAPI) AdminCall(r *http.Request, rawTx, result *string) (err error) {
	if !params.HasRouterAdmin() {
		return fmt.Errorf("no admin is configed")
	}
	tx, err := admin.DecodeTransaction(*rawTx)
	if err != nil {
		return err
	}
	sender, args, err := admin.VerifyTransaction(tx)
	if err != nil {
		return err
	}
	senderAddress := sender.String()
	if !params.IsRouterAdmin(senderAddress) {
		switch args.Method {
		case reswapCmd, passForbiddenSwapoutCmd:
			return fmt.Errorf("sender %v is not admin", senderAddress)
		case maintainCmd:
			action := args.Params[0]
			switch action {
			case actPause, actUnpause:
				return fmt.Errorf("sender %v is not admin", senderAddress)
			}
		case passbigvalueCmd, replaceswapCmd, forbidSwapCmd:
		default:
			return fmt.Errorf("unknown admin method '%v'", args.Method)
		}
		if !params.IsRouterAssistant(senderAddress) {
			return fmt.Errorf("sender %v is not assistant", senderAddress)
		}
	}
	log.Info("admin call", "caller", senderAddress, "args", args, "result", result)
	return doRouterAdminCall(args, result)
}

func doRouterAdminCall(args *admin.CallArgs, result *string) error {
	switch args.Method {
	case maintainCmd:
		return maintain(args, result)
	case passbigvalueCmd:
		return routerPassBigValue(args, result)
	case reswapCmd:
		return routerReswap(args, result)
	case replaceswapCmd:
		return routerReplaceSwap(args, result)
	case forbidSwapCmd:
		return routerForbidSwap(args, result)
	case passForbiddenSwapoutCmd:
		return routerPassForbiddenSwapout(args, result)
	default:
		return fmt.Errorf("unknown admin method '%v'", args.Method)
	}
}

func getKeys(args *admin.CallArgs, startPos int) (chainID, txid string, logIndex int, err error) {
	if len(args.Params) < startPos+3 {
		err = fmt.Errorf("wrong number of params, have %v want at least %v", len(args.Params), startPos+3)
		return
	}
	chainID = args.Params[startPos]
	if _, err = common.GetBigIntFromStr(chainID); err != nil || chainID == "" {
		err = fmt.Errorf("wrong chain id '%v'", chainID)
		return
	}
	txid = args.Params[startPos+1]
	if txid == "" || (common.HasHexPrefix(txid) && !common.IsHexHash(txid)) {
		err = fmt.Errorf("wrong tx id '%v'", txid)
		return
	}
	logIndexStr := args.Params[startPos+2]
	logIndex, err = common.GetIntFromStr(logIndexStr)
	if err != nil {
		err = fmt.Errorf("wrong log index '%v'", logIndexStr)
	}
	return
}

//nolint:gocyclo // allow big switch
func maintain(args *admin.CallArgs, result *string) (err error) {
	if len(args.Params) != 2 {
		return fmt.Errorf("wrong number of params, have %v want 2", len(args.Params))
	}
	action := args.Params[0]
	arguments := args.Params[1]

	switch action {
	case actPause, actUnpause:
		chainIDs := strings.Split(arguments, ",")
		if action == actPause {
			router.AddPausedChainIDs(chainIDs)
		} else {
			router.RemovePausedChainIDs(chainIDs)
		}
		log.Infof("after action %v, the paused chainIDs are %v", action, router.GetPausedChainIDs())
	case actWhitelist, actUnwhitelist:
		isAdd := strings.EqualFold(action, actWhitelist)
		args := strings.Split(arguments, ",")
		if len(args) < 3 {
			return fmt.Errorf("miss arguments")
		}
		whitelistType := strings.ToLower(args[0])
		switch whitelistType {
		case "callbycontract":
			params.AddOrRemoveCallByContractWhitelist(args[1], args[2:], isAdd)
		case "callbycontractcodehash":
			params.AddOrRemoveCallByContractCodeHashWhitelist(args[1], args[2:], isAdd)
		case "bigvalue":
			params.AddOrRemoveBigValueWhitelist(args[1], args[2:], isAdd)
		default:
			return fmt.Errorf("unknown whitelist type '%v'", whitelistType)
		}
	case actBlacklist, actUnblacklist:
		isAdd := strings.EqualFold(action, actBlacklist)
		args := strings.Split(arguments, ",")
		if len(args) < 2 {
			return fmt.Errorf("miss arguments")
		}
		blacklistType := strings.ToLower(args[0])
		switch blacklistType {
		case "chainid":
			params.AddOrRemoveChainIDBlackList(args[1:], isAdd)
		case "tokenid":
			params.AddOrRemoveTokenIDBlackList(args[1:], isAdd)
		case "account":
			params.AddOrRemoveAccountBlackList(args[1:], isAdd)
		default:
			return fmt.Errorf("unknown blacklist type '%v'", blacklistType)
		}
	default:
		return fmt.Errorf("unknown maintain action '%v'", action)
	}
	*result = successReuslt
	return nil
}

func getGasPrice(args *admin.CallArgs, startPos int) (gasPrice *big.Int, err error) {
	if len(args.Params) < startPos+1 {
		err = fmt.Errorf("wrong number of params, have %v want at least %v", len(args.Params), startPos+3)
		return
	}
	gasPriceStr := args.Params[startPos]
	if gasPriceStr == "" {
		return
	}
	if gasPrice, err = common.GetBigIntFromStr(gasPriceStr); err != nil {
		err = fmt.Errorf("wrong gas price '%v'", gasPriceStr)
	}
	return
}

func routerPassBigValue(args *admin.CallArgs, result *string) (err error) {
	chainID, txid, logIndex, err := getKeys(args, 0)
	if err != nil {
		return err
	}
	bridge := router.GetBridgeByChainID(chainID)
	if bridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	verifyArgs := &tokens.VerifyArgs{
		SwapType:      tokens.ERC20SwapType,
		LogIndex:      logIndex,
		AllowUnstable: false,
	}
	swapInfo, err := bridge.VerifyTransaction(txid, verifyArgs)
	if err != nil {
		return err
	}
	err = mongodb.RouterAdminPassBigValue(chainID, txid, logIndex)
	if err != nil {
		return err
	}
	_ = worker.AddInitialSwapResult(swapInfo, mongodb.MatchTxEmpty)
	*result = successReuslt
	return nil
}

func routerReswap(args *admin.CallArgs, result *string) (err error) {
	chainID, txid, logIndex, err := getKeys(args, 0)
	if err != nil {
		return err
	}
	err = mongodb.RouterAdminReswap(chainID, txid, logIndex)
	if err != nil {
		return err
	}
	worker.DeleteCachedSwap(chainID, txid, logIndex)
	*result = successReuslt
	return nil
}

func routerReplaceSwap(args *admin.CallArgs, result *string) (err error) {
	chainID, txid, logIndex, err := getKeys(args, 0)
	if err != nil {
		return err
	}
	gasPrice, err := getGasPrice(args, 3)
	if err != nil {
		return err
	}
	res, err := mongodb.FindRouterSwapResult(chainID, txid, logIndex)
	if err != nil {
		return err
	}
	err = worker.ReplaceRouterSwap(res, gasPrice, true)
	if err != nil {
		return err
	}
	*result = successReuslt
	return nil
}

func routerForbidSwap(args *admin.CallArgs, result *string) (err error) {
	chainID, txid, logIndex, err := getKeys(args, 0)
	if err != nil {
		return err
	}
	var memo string
	if len(args.Params) > 3 {
		memo = args.Params[3]
	}
	err1 := mongodb.UpdateRouterSwapResultStatus(
		chainID, txid, logIndex,
		mongodb.ManualMakeFail,
		time.Now().Unix(), memo,
	)
	err2 := mongodb.UpdateRouterSwapStatus(
		chainID, txid, logIndex,
		mongodb.ManualMakeFail,
		time.Now().Unix(), memo,
	)
	if err1 != nil && err2 != nil {
		return err1
	}
	*result = successReuslt
	return nil
}

func routerPassForbiddenSwapout(args *admin.CallArgs, result *string) (err error) {
	chainID, txid, logIndex, err := getKeys(args, 0)
	if err != nil {
		return err
	}
	bridge := router.GetBridgeByChainID(chainID)
	if bridge == nil {
		return tokens.ErrNoBridgeForChainID
	}
	verifyArgs := &tokens.VerifyArgs{
		SwapType:      tokens.ERC20SwapType,
		LogIndex:      logIndex,
		AllowUnstable: false,
	}
	_, err = bridge.VerifyTransaction(txid, verifyArgs)
	if !errors.Is(err, tokens.ErrSwapoutForbidden) {
		return fmt.Errorf("verify error mismatch, %v", err)
	}
	err = mongodb.RouterAdminPassForbiddenSwapout(chainID, txid, logIndex)
	if err != nil {
		return err
	}
	*result = successReuslt
	return nil
}

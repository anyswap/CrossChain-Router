package swapapi

import (
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	rpcjson "github.com/gorilla/rpc/v2/json2"
)

func newRPCError(ec rpcjson.ErrorCode, message string) error {
	return &rpcjson.Error{
		Code:    ec,
		Message: message,
	}
}

func newRPCInternalError(err error) error {
	return newRPCError(-32000, "rpcError: "+err.Error())
}

// GetServerInfo get server info
func GetServerInfo() *ServerInfo {
	return &ServerInfo{
		Identifier:     params.GetIdentifier(),
		Version:        params.VersionWithMeta,
		ConfigContract: params.GetOnchainContract(),
	}
}

// RegisterRouterSwap register router swap
// if logIndex is 0 then check all logs, otherwise only check the specified log
func RegisterRouterSwap(fromChainID, txid, logIndexStr string) (*MapIntResult, error) {
	return registerSwap(fromChainID, txid, logIndexStr, tokens.RouterSwapType)
}

// RegisterAnyCallSwap register anycall swap
// if logIndex is 0 then check all logs, otherwise only check the specified log
func RegisterAnyCallSwap(fromChainID, txid, logIndexStr string) (*MapIntResult, error) {
	return registerSwap(fromChainID, txid, logIndexStr, tokens.AnyCallSwapType)
}

func registerSwap(fromChainID, txid, logIndexStr string, swapType tokens.SwapType) (*MapIntResult, error) {
	log.Info("[api] register swap", "chainid", fromChainID, "txid", txid, "logIndex", logIndexStr, "swapType", swapType.String())
	chainID, err := common.GetBigIntFromStr(fromChainID)
	if err != nil {
		return nil, newRPCInternalError(err)
	}
	logIndex, err := getLogIndex(logIndexStr)
	if err != nil {
		return nil, err
	}
	bridge := router.GetBridgeByChainID(chainID.String())
	if bridge == nil {
		return nil, newRPCInternalError(tokens.ErrNoBridgeForChainID)
	}
	result := MapIntResult(make(map[int]string))
	registerArgs := &tokens.RegisterArgs{
		SwapType: swapType,
		LogIndex: logIndex,
	}
	swapInfos, errs := bridge.RegisterSwap(txid, registerArgs)
	for i, swapInfo := range swapInfos {
		verifyErr := errs[i]
		logIndex := swapInfo.LogIndex
		if !tokens.ShouldRegisterRouterSwapForError(verifyErr) {
			result[logIndex] = "failed: " + verifyErr.Error()
			continue
		}
		oldSwap, registeredOk := getRegisteredRouterSwap(fromChainID, txid, logIndex)
		if registeredOk {
			result[logIndex] = "alreday registered"
			continue
		}
		result[logIndex] = "success"
		newStatus := mongodb.GetRouterSwapStatusByVerifyError(verifyErr)
		if oldSwap == nil {
			var memo string
			if verifyErr != nil {
				memo = verifyErr.Error()
			}
			err = addMgoSwap(swapInfo, newStatus, memo)
		} else if newStatus != oldSwap.Status {
			log.Info("update swap status", "chainid", fromChainID, "txid", txid, "logIndex", logIndexStr, "oldStatus", oldSwap.Status, "newStatus", newStatus)
			err = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, newStatus, time.Now().Unix(), "")
		}
		if err != nil {
			log.Info("register swap db error", "chainid", fromChainID, "txid", txid, "logIndex", logIndexStr, "err", err)
		}
	}
	return &result, nil
}

func getRegisteredRouterSwap(fromChainID, txid string, logIndex int) (oldSwap *mongodb.MgoSwap, registeredOk bool) {
	oldSwap, _ = mongodb.FindRouterSwap(fromChainID, txid, logIndex)
	if oldSwap == nil {
		return nil, false
	}
	if oldSwap.Status.IsRegisteredOk() {
		return oldSwap, true
	}
	oldSwapRes, _ := mongodb.FindRouterSwapResult(fromChainID, txid, logIndex)
	if oldSwapRes != nil && oldSwapRes.SwapTx != "" {
		return oldSwap, true
	}
	return oldSwap, false
}

func addMgoSwap(swapInfo *tokens.SwapTxInfo, status mongodb.SwapStatus, memo string) (err error) {
	swap := &mongodb.MgoSwap{
		SwapType:    uint32(swapInfo.SwapType),
		TxID:        swapInfo.Hash,
		TxTo:        swapInfo.TxTo,
		Bind:        swapInfo.Bind,
		LogIndex:    swapInfo.LogIndex,
		FromChainID: swapInfo.FromChainID.String(),
		ToChainID:   swapInfo.ToChainID.String(),
		Status:      status,
		Timestamp:   time.Now().Unix(),
		Memo:        memo,
	}
	swap.SwapInfo = mongodb.ConvertToSwapInfo(&swapInfo.SwapInfo)
	err = mongodb.AddRouterSwap(swap)
	if err != nil {
		log.Warn("[api] add router swap", "swap", swap, "err", err)
	} else {
		log.Info("[api] add router swap", "swap", swap)
	}
	return err
}

func getLogIndex(logindexStr string) (int, error) {
	if logindexStr == "" {
		return 0, nil
	}
	logIndex, err := common.GetIntFromStr(logindexStr)
	if err != nil {
		return 0, newRPCInternalError(err)
	}
	if logIndex < 0 {
		return 0, newRPCError(-32099, "negative log index")
	}
	return logIndex, nil
}

// GetRouterSwap impl
func GetRouterSwap(fromChainID, txid, logindexStr string) (*SwapInfo, error) {
	logindex, err := getLogIndex(logindexStr)
	if err != nil {
		return nil, err
	}
	result, err := mongodb.FindRouterSwapResult(fromChainID, txid, logindex)
	if err == nil {
		return ConvertMgoSwapResultToSwapInfo(result), nil
	}
	register, err := mongodb.FindRouterSwap(fromChainID, txid, logindex)
	if err == nil {
		return ConvertMgoSwapToSwapInfo(register), nil
	}
	return nil, mongodb.ErrSwapNotFound
}

// GetRouterSwapHistory impl
func GetRouterSwapHistory(fromChainID, address string, offset, limit int, status string) ([]*SwapInfo, error) {
	switch {
	case limit == 0:
		limit = 20 // default
	case limit > 100:
		limit = 100
	case limit < -100:
		limit = -100
	}
	result, err := mongodb.FindRouterSwapResults(fromChainID, address, offset, limit, status)
	if err != nil {
		return nil, err
	}
	return ConvertMgoSwapResultsToSwapInfos(result), nil
}

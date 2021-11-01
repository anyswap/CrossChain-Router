package swapapi

import (
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	rpcjson "github.com/gorilla/rpc/v2/json2"
)

var (
	oraclesInfo sync.Map // string -> *OracleInfo // key is enode
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

// GetOracleInfo get oracle info
func GetOracleInfo() map[string]*OracleInfo {
	result := make(map[string]*OracleInfo, 4)
	oraclesInfo.Range(func(k, v interface{}) bool {
		enode := k.(string)
		startIndex := strings.Index(enode, "enode://")
		endIndex := strings.Index(enode, "@")
		if startIndex != -1 && endIndex != -1 {
			info := v.(*OracleInfo)
			enodeID := enode[startIndex+8 : endIndex]
			result[strings.ToLower(enodeID)] = info
		}
		return true
	})
	return result
}

// ReportOracleInfo report oracle info
func ReportOracleInfo(oracle string, info *OracleInfo) error {
	var exist bool
	for _, enode := range mpc.GetAllEnodes() {
		if strings.EqualFold(oracle, enode) {
			if !strings.EqualFold(oracle, mpc.GetSelfEnode()) {
				exist = true
			}
			break
		}
	}
	if !exist {
		return newRPCError(-32000, "wrong oracle info")
	}

	key := strings.ToLower(oracle)
	if val, exist := oraclesInfo.Load(key); exist {
		oldInfo := val.(*OracleInfo)
		oldTime := oldInfo.HeartbeatTimestamp
		if info.HeartbeatTimestamp > oldTime &&
			info.HeartbeatTimestamp < time.Now().Unix()+60 {
			oraclesInfo.Store(key, info)
		}
	} else {
		oraclesInfo.Store(key, info)
	}
	return nil
}

// RegisterRouterSwap register router swap
// if logIndex is 0 then check all logs, otherwise only check the specified log
func RegisterRouterSwap(fromChainID, txid, logIndexStr string) (*MapIntResult, error) {
	swapType := tokens.GetRouterSwapType()
	log.Debug("[api] register swap", "chainid", fromChainID, "txid", txid, "logIndex", logIndexStr, "swapType", swapType.String())
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
			result[logIndex] = "already registered"
			continue
		}
		result[logIndex] = "success"
		newStatus := mongodb.GetRouterSwapStatusByVerifyError(verifyErr)
		var memo string
		if verifyErr != nil {
			memo = verifyErr.Error()
		}
		if oldSwap == nil {
			err = addMgoSwap(swapInfo, newStatus, memo)
		} else if newStatus != oldSwap.Status {
			mgoSwapInfo := mongodb.ConvertToSwapInfo(&swapInfo.SwapInfo)
			log.Info("[register] update swap info and status", "chainid", fromChainID, "txid", txid, "logIndex", logIndexStr, "oldStatus", oldSwap.Status, "newStatus", newStatus, "swapinfo", mgoSwapInfo)
			err = mongodb.UpdateRouterSwapInfoAndStatus(fromChainID, txid, logIndex, &mgoSwapInfo, newStatus, time.Now().Unix(), memo)
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
	if oldSwapRes != nil {
		return oldSwap, true
	}
	return oldSwap, false
}

func addMgoSwap(swapInfo *tokens.SwapTxInfo, status mongodb.SwapStatus, memo string) (err error) {
	swap := &mongodb.MgoSwap{
		SwapType:    uint32(swapInfo.SwapType),
		TxID:        swapInfo.Hash,
		TxTo:        swapInfo.TxTo,
		From:        swapInfo.From,
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

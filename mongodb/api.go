package mongodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	allChainIDs  = "all"
	allAddresses = "all"
)

var (
	retryLock            sync.Mutex
	verifyLock           sync.Mutex
	updateResultLock     sync.Mutex
	updateOldSwapTxsLock sync.Mutex

	maxCountOfResults = int64(1000)
)

// GetRouterSwapKey get router swap key
func GetRouterSwapKey(fromChainID, txid string, logindex int) string {
	return strings.ToLower(fmt.Sprintf("%v:%v:%v", fromChainID, txid, logindex))
}

// AddRouterSwap add router swap
func AddRouterSwap(ms *MgoSwap) error {
	ms.Key = GetRouterSwapKey(ms.FromChainID, ms.TxID, ms.LogIndex)
	ms.InitTime = common.NowMilli()
	_, err := collRouterSwap.InsertOne(clientCtx, ms)
	switch {
	case err == nil:
		log.Info("mongodb add router swap success", "chainid", ms.FromChainID, "txid", ms.TxID, "logindex", ms.LogIndex)
	case !mongo.IsDuplicateKeyError(err):
		log.Error("mongodb add router swap failed", "chainid", ms.FromChainID, "txid", ms.TxID, "logindex", ms.LogIndex, "err", err)
	default:
		swap := &MgoSwap{}
		errt := collRouterSwap.FindOne(clientCtx, bson.M{"_id": ms.Key}).Decode(swap)
		if errt == nil && swap.Status == TxNotSwapped {
			now := time.Now().Unix()
			if swap.Timestamp+3*24*3600 < now {
				_, _ = collRouterSwap.UpdateByID(clientCtx, ms.Key, bson.M{"$set": bson.M{"timestamp": now}})
			}
		}
	}
	return mgoError(err)
}

// PassRouterSwapVerify pass router swap verify
func PassRouterSwapVerify(fromChainID, txid string, logindex int, timestamp int64) error {
	verifyLock.Lock()
	defer verifyLock.Unlock()

	swap, err := FindRouterSwap(fromChainID, txid, logindex)
	if err != nil {
		return fmt.Errorf("forbid pass verify as swap is not exist")
	}
	if swap.Status != TxNotStable {
		return fmt.Errorf("forbid pass verify as swap status is '%v'", swap.Status)
	}

	key := GetRouterSwapKey(fromChainID, txid, logindex)
	updates := bson.M{"status": TxNotSwapped, "timestamp": timestamp}
	_, err = collRouterSwap.UpdateByID(clientCtx, key, bson.M{"$set": updates})
	if err == nil {
		log.Info("mongodb pass verify success", "chainid", fromChainID, "txid", txid, "logindex", logindex)
	} else {
		log.Error("mongodb pass verify failed", "chainid", fromChainID, "txid", txid, "logindex", logindex, "err", err)
	}
	return mgoError(err)
}

// UpdateRouterSwapStatus update router swap status
func UpdateRouterSwapStatus(fromChainID, txid string, logindex int, status SwapStatus, timestamp int64, memo string) error {
	if status == TxNotStable {
		return errors.New("forbid update swap status to TxNotStable")
	}
	key := GetRouterSwapKey(fromChainID, txid, logindex)
	updates := bson.M{"status": status, "timestamp": timestamp}
	if memo != "" {
		updates["memo"] = memo
	} else if status == TxNotSwapped {
		updates["memo"] = ""
	}
	_, err := collRouterSwap.UpdateByID(clientCtx, key, bson.M{"$set": updates})
	if err == nil {
		logFunc := log.GetPrintFuncOr(func() bool { return status == TxVerifyFailed }, log.Warn, log.Info)
		logFunc("mongodb update router swap status success", "chainid", fromChainID, "txid", txid, "logindex", logindex, "status", status)
	} else {
		log.Error("mongodb update router swap status failed", "chainid", fromChainID, "txid", txid, "logindex", logindex, "status", status, "err", err)
	}
	return mgoError(err)
}

// UpdateRouterSwapInfoAndStatus update router swap info and status
func UpdateRouterSwapInfoAndStatus(fromChainID, txid string, logindex int, swapInfo *SwapInfo, status SwapStatus, timestamp int64, memo string) error {
	retryLock.Lock()
	defer retryLock.Unlock()

	key := GetRouterSwapKey(fromChainID, txid, logindex)

	swap, err := FindRouterSwap(fromChainID, txid, logindex)
	if err != nil {
		return fmt.Errorf("forbid update swap info if swap is not exist")
	}
	if swap.Status.IsRegisteredOk() {
		return fmt.Errorf("forbid update swap info from registered status %v", swap.Status.String())
	}

	result := &MgoSwapResult{}
	err = collRouterSwapResult.FindOne(clientCtx, bson.M{"_id": key}).Decode(result)
	if err == nil {
		return fmt.Errorf("forbid update swap info if swap result exists")
	}

	updates := bson.M{
		"swapinfo":  *swapInfo,
		"status":    status,
		"timestamp": timestamp,
		"inittime":  timestamp * 1000,
		"memo":      memo,
	}

	_, err = collRouterSwap.UpdateByID(clientCtx, key, bson.M{"$set": updates})
	if err == nil {
		log.Info("mongodb update router swap info and status success", "chainid", fromChainID, "txid", txid, "logindex", logindex, "status", status, "swapinfo", swapInfo)
	} else {
		log.Error("mongodb update router swap info and status failed", "chainid", fromChainID, "txid", txid, "logindex", logindex, "status", status, "swapinfo", swapInfo, "err", err)
	}
	return mgoError(err)
}

// FindRouterSwap find router swap
func FindRouterSwap(fromChainID, txid string, logindex int) (*MgoSwap, error) {
	key := GetRouterSwapKey(fromChainID, txid, logindex)
	result := &MgoSwap{}
	err := collRouterSwap.FindOne(clientCtx, bson.M{"_id": key}).Decode(result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// FindRouterSwapAuto find router swap
func FindRouterSwapAuto(fromChainID, txid string, logindex int) (*MgoSwap, error) {
	if logindex == 0 {
		return findFirstRouterSwap(fromChainID, txid)
	}
	return FindRouterSwap(fromChainID, txid, logindex)
}

func findFirstRouterSwap(fromChainID, txid string) (*MgoSwap, error) {
	result := &MgoSwap{}
	query := getChainAndTxIDQuery(fromChainID, txid)
	err := collRouterSwap.FindOne(clientCtx, query).Decode(result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

func getChainAndTxIDQuery(fromChainID, txid string) bson.M {
	qtxid := bson.M{"txid": bson.M{"$regex": primitive.Regex{Pattern: txid, Options: "i"}}}
	qchainid := bson.M{"fromChainID": fromChainID}
	return bson.M{"$and": []bson.M{qtxid, qchainid}}
}

func getStatusQuery(status SwapStatus, septime int64) bson.M {
	qtime := bson.M{"timestamp": bson.M{"$gte": septime}}
	qstatus := bson.M{"status": status}
	queries := []bson.M{qtime, qstatus}
	return bson.M{"$and": queries}
}

func getStatusQueryWithChainID(fromChainID string, status SwapStatus, septime int64) bson.M {
	qtime := bson.M{"timestamp": bson.M{"$gte": septime}}
	qstatus := bson.M{"status": status}
	qchainid := bson.M{"fromChainID": fromChainID}
	queries := []bson.M{qtime, qstatus, qchainid}
	return bson.M{"$and": queries}
}

// FindRouterSwapsWithStatus find router swap with status
func FindRouterSwapsWithStatus(status SwapStatus, septime int64) ([]*MgoSwap, error) {
	query := getStatusQuery(status, septime)
	opts := &options.FindOptions{
		Sort:  bson.D{{Key: "inittime", Value: 1}},
		Limit: &maxCountOfResults,
	}
	cur, err := collRouterSwap.Find(clientCtx, query, opts)
	if err != nil {
		return nil, mgoError(err)
	}
	result := make([]*MgoSwap, 0, 20)
	err = cur.All(clientCtx, &result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// FindRouterSwapsWithChainIDAndStatus find router swap with chainid and status in the past septime
//nolint:dupl // allow duplicate
func FindRouterSwapsWithChainIDAndStatus(fromChainID string, status SwapStatus, septime int64) ([]*MgoSwap, error) {
	query := getStatusQueryWithChainID(fromChainID, status, septime)
	opts := &options.FindOptions{
		Sort:  bson.D{{Key: "inittime", Value: 1}},
		Limit: &maxCountOfResults,
	}
	cur, err := collRouterSwap.Find(clientCtx, query, opts)
	if err != nil {
		return nil, mgoError(err)
	}
	result := make([]*MgoSwap, 0, 20)
	err = cur.All(clientCtx, &result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// AddRouterSwapResult add router swap result
func AddRouterSwapResult(mr *MgoSwapResult) error {
	mr.Key = GetRouterSwapKey(mr.FromChainID, mr.TxID, mr.LogIndex)
	mr.InitTime = common.NowMilli()
	_, err := collRouterSwapResult.InsertOne(clientCtx, mr)
	if err == nil {
		log.Info("mongodb add router swap result success", "chainid", mr.FromChainID, "txid", mr.TxID, "logindex", mr.LogIndex)
	} else if !mongo.IsDuplicateKeyError(err) {
		log.Error("mongodb add router swap result failed", "chainid", mr.FromChainID, "txid", mr.TxID, "logindex", mr.LogIndex, "err", err)
	}
	return mgoError(err)
}

// AllocateRouterSwapNonce allocate swap nonce (for parallel signing)
func AllocateRouterSwapNonce(args *tokens.BuildTxArgs, nonceptr *uint64, isRecycleNonce bool) (swapnonce uint64, err error) {
	updateResultLock.Lock()
	defer updateResultLock.Unlock()

	fromChainID := args.FromChainID.String()
	txid := args.SwapID
	logindex := args.LogIndex

	swapnonce = *nonceptr
	if isRecycleNonce && swapnonce == 0 {
		return 0, errors.New("swap nonce is alreay recycled")
	}

	err = checkRouterSwapResultUpdate(fromChainID, txid, logindex, swapnonce)
	if err != nil {
		return 0, err
	}

	key := GetRouterSwapKey(fromChainID, txid, logindex)
	nowTime := time.Now().Unix()

	resUpdates := bson.M{
		"mpc":       args.From,
		"status":    MatchTxNotStable,
		"swapnonce": swapnonce,
		"timestamp": nowTime,
	}
	if args.SwapValue != nil {
		resUpdates["swapvalue"] = args.SwapValue.String()
	}
	_, err = collRouterSwapResult.UpdateByID(clientCtx, key, bson.M{"$set": resUpdates})
	if err != nil {
		log.Warn("mongodb allocate swap nonce failed", "chainid", fromChainID, "txid", txid, "logindex", logindex, "swapnonce", swapnonce, "err", err)
		return 0, mgoError(err)
	}

	log.Info("mongodb allocate swap nonce success", "chainid", fromChainID, "txid", txid, "logindex", logindex, "swapnonce", swapnonce)

	statusUpdates := bson.M{"status": TxProcessed, "timestamp": nowTime}
	_, errf := collRouterSwap.UpdateByID(clientCtx, key, bson.M{"$set": statusUpdates})
	if errf != nil {
		log.Warn("mongodb update swap status to TxProcessed failed", "chainid", fromChainID, "txid", txid, "logindex", logindex, "swapnonce", swapnonce, "err", errf)
	}

	if isRecycleNonce {
		*nonceptr = 0
	} else {
		*nonceptr++
	}
	return swapnonce, nil
}

// UpdateRouterSwapResultStatus update router swap result status
func UpdateRouterSwapResultStatus(fromChainID, txid string, logindex int, status SwapStatus, timestamp int64, memo string) error {
	key := GetRouterSwapKey(fromChainID, txid, logindex)
	updates := bson.M{"status": status, "timestamp": timestamp}
	if memo != "" {
		updates["memo"] = memo
	}
	if status == Reswapping {
		updates["memo"] = ""
		updates["swaptx"] = ""
		updates["oldswaptxs"] = nil
		updates["swapheight"] = 0
		updates["swaptime"] = 0
		updates["swapnonce"] = 0
	}
	_, err := collRouterSwapResult.UpdateByID(clientCtx, key, bson.M{"$set": updates})
	if err == nil {
		log.Info("mongodb update swap result status success", "chainid", fromChainID, "txid", txid, "logindex", logindex, "status", status)
	} else {
		log.Error("mongodb update swap result status failed", "chainid", fromChainID, "txid", txid, "logindex", logindex, "status", status, "err", err)
	}
	return mgoError(err)
}

// UpdateRouterOldSwapTxs update old swaptxs by appending `swapTx`
func UpdateRouterOldSwapTxs(fromChainID, txid string, logindex int, swapTx string) error {
	if swapTx == "" {
		return nil
	}
	updateOldSwapTxsLock.Lock()
	defer updateOldSwapTxsLock.Unlock()

	swapRes, err := FindRouterSwapResult(fromChainID, txid, logindex)
	if err != nil {
		return err
	}

	// already exist
	if strings.EqualFold(swapTx, swapRes.SwapTx) {
		return nil
	}
	for _, oldSwapTx := range swapRes.OldSwapTxs {
		if strings.EqualFold(swapTx, oldSwapTx) {
			return nil
		}
	}

	var updates bson.M

	if len(swapRes.OldSwapTxs) == 0 {
		updateSet := bson.M{
			"swaptx":     swapTx,
			"oldswaptxs": []string{swapRes.SwapTx, swapTx},
			"timestamp":  time.Now().Unix(),
		}
		updates = bson.M{"$set": updateSet}
	} else {
		updates = bson.M{
			"$set":  bson.M{"swaptx": swapTx, "timestamp": time.Now().Unix()},
			"$push": bson.M{"oldswaptxs": swapTx},
		}
	}

	key := GetRouterSwapKey(fromChainID, txid, logindex)
	_, err = collRouterSwapResult.UpdateByID(clientCtx, key, updates)
	if err == nil {
		log.Info("UpdateRouterOldSwapTxs success", "fromChainID", fromChainID, "txid", txid, "logIndex", logindex, "swaptx", swapTx, "nonce", swapRes.SwapNonce)
	} else {
		log.Error("UpdateRouterOldSwapTxs failed", "fromChainID", fromChainID, "txid", txid, "logIndex", logindex, "swaptx", swapTx, "nonce", swapRes.SwapNonce, "err", err)
	}
	return mgoError(err)
}

// FindRouterSwapResult find router swap result
func FindRouterSwapResult(fromChainID, txid string, logindex int) (*MgoSwapResult, error) {
	key := GetRouterSwapKey(fromChainID, txid, logindex)
	result := &MgoSwapResult{}
	err := collRouterSwapResult.FindOne(clientCtx, bson.M{"_id": key}).Decode(result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// FindRouterSwapResultAuto find router swap result
func FindRouterSwapResultAuto(fromChainID, txid string, logindex int) (*MgoSwapResult, error) {
	if logindex == 0 {
		return findFirstRouterSwapResult(fromChainID, txid)
	}
	return FindRouterSwapResult(fromChainID, txid, logindex)
}

func findFirstRouterSwapResult(fromChainID, txid string) (*MgoSwapResult, error) {
	result := &MgoSwapResult{}
	query := getChainAndTxIDQuery(fromChainID, txid)
	err := collRouterSwapResult.FindOne(clientCtx, query).Decode(result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// FindRouterSwapResultsWithStatus find router swap result with status
func FindRouterSwapResultsWithStatus(status SwapStatus, septime int64) ([]*MgoSwapResult, error) {
	query := getStatusQuery(status, septime)
	opts := &options.FindOptions{
		Sort:  bson.D{{Key: "inittime", Value: 1}},
		Limit: &maxCountOfResults,
	}
	cur, err := collRouterSwapResult.Find(clientCtx, query, opts)
	if err != nil {
		return nil, mgoError(err)
	}
	result := make([]*MgoSwapResult, 0, 20)
	err = cur.All(clientCtx, &result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// FindRouterSwapResultsWithChainIDAndStatus find router swap result with chainid and status in the past septime
//nolint:dupl // allow duplicate
func FindRouterSwapResultsWithChainIDAndStatus(fromChainID string, status SwapStatus, septime int64) ([]*MgoSwapResult, error) {
	query := getStatusQueryWithChainID(fromChainID, status, septime)
	opts := &options.FindOptions{
		Sort:  bson.D{{Key: "inittime", Value: 1}},
		Limit: &maxCountOfResults,
	}
	cur, err := collRouterSwapResult.Find(clientCtx, query, opts)
	if err != nil {
		return nil, mgoError(err)
	}
	result := make([]*MgoSwapResult, 0, 20)
	err = cur.All(clientCtx, &result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// FindNextSwapNonce find next swap nonce
func FindNextSwapNonce(chainID, mpc string) (uint64, error) {
	qchainid := bson.M{"toChainID": chainID}
	qmpc := bson.M{"mpc": bson.M{"$regex": primitive.Regex{Pattern: mpc, Options: "i"}}}
	queries := []bson.M{qchainid, qmpc}
	opts := &options.FindOneOptions{
		Sort: bson.D{{Key: "swapnonce", Value: -1}},
	}
	result := &MgoSwapResult{}
	err := collRouterSwapResult.FindOne(clientCtx, bson.M{"$and": queries}, opts).Decode(result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, nil
		}
		log.Error("FindNextSwapNonce failed", "chainID", chainID, "mpc", mpc, "err", err)
		return 0, mgoError(err)
	}
	log.Info("FindNextSwapNonce success", "chainID", chainID, "mpc", mpc, "nonce", result.SwapNonce)
	return result.SwapNonce + 1, nil
}

// FindRouterSwapResultsToStable find swap results to stable
func FindRouterSwapResultsToStable(chainID string, septime int64) ([]*MgoSwapResult, error) {
	qtime := bson.M{"inittime": bson.M{"$gte": septime}}
	qstatus := bson.M{"status": MatchTxNotStable}
	qchainid := bson.M{"toChainID": chainID}
	queries := []bson.M{qtime, qstatus, qchainid}

	limit := int64(100)
	opts := &options.FindOptions{
		Sort:  bson.D{{Key: "swapnonce", Value: 1}},
		Limit: &limit,
	}
	cur, err := collRouterSwapResult.Find(clientCtx, bson.M{"$and": queries}, opts)
	if err != nil {
		return nil, mgoError(err)
	}
	result := make([]*MgoSwapResult, 0, limit)
	err = cur.All(clientCtx, &result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// FindRouterSwapResultsToReplace find router swap result with status
func FindRouterSwapResultsToReplace(chainID string, septime int64) ([]*MgoSwapResult, error) {
	qtime := bson.M{"inittime": bson.M{"$gte": septime}}
	qstatus := bson.M{"status": MatchTxNotStable}
	qchainid := bson.M{"toChainID": chainID}
	qheight := bson.M{"swapheight": 0}
	queries := []bson.M{qtime, qstatus, qchainid, qheight}

	limit := int64(20)
	opts := &options.FindOptions{
		Sort:  bson.D{{Key: "swapnonce", Value: 1}},
		Limit: &limit,
	}
	cur, err := collRouterSwapResult.Find(clientCtx, bson.M{"$and": queries}, opts)
	if err != nil {
		return nil, mgoError(err)
	}
	result := make([]*MgoSwapResult, 0, limit)
	err = cur.All(clientCtx, &result)
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

func getStatusesFromStr(status string) (registerStatuses, resultStatuses []SwapStatus) {
	parts := strings.Split(status, ",")
	registerStatuses = make([]SwapStatus, 0, 5)
	resultStatuses = make([]SwapStatus, 0, 5)
	for _, part := range parts {
		if part == "" {
			continue
		}
		num, err := common.GetUint64FromStr(part)
		if err == nil {
			swapStatus := SwapStatus(num)
			if swapStatus.IsResultStatus() {
				resultStatuses = append(resultStatuses, swapStatus)
			} else {
				registerStatuses = append(registerStatuses, swapStatus)
			}
		}
	}
	return registerStatuses, resultStatuses
}

// FindRouterSwapResults find router swap results with chainid and address
//nolint:gocyclo // allow long method
func FindRouterSwapResults(fromChainID, address string, offset, limit int, status string) ([]*MgoSwapResult, error) {
	var queries []bson.M

	if address != "" && address != allAddresses {
		qaddress := bson.M{"from": bson.M{"$regex": primitive.Regex{Pattern: address, Options: "i"}}}
		queries = append(queries, qaddress)
	}

	if fromChainID != "" && fromChainID != allChainIDs {
		queries = append(queries, bson.M{"fromChainID": fromChainID})
	}

	registerStatuses, resultStatuses := getStatusesFromStr(status)
	filterStatuses, isInResultColl := resultStatuses, true
	if len(resultStatuses) == 0 && len(registerStatuses) > 0 {
		filterStatuses = registerStatuses
		isInResultColl = false
	}
	if len(filterStatuses) > 0 {
		if len(filterStatuses) == 1 {
			queries = append(queries, bson.M{"status": filterStatuses[0]})
		} else {
			qstatus := bson.M{"status": bson.M{"$in": filterStatuses}}
			queries = append(queries, qstatus)
		}
	}

	opts := &options.FindOptions{}
	if limit >= 0 {
		opts = opts.SetSort(bson.D{{Key: "inittime", Value: 1}}).
			SetSkip(int64(offset)).SetLimit(int64(limit))
	} else {
		opts = opts.SetSort(bson.D{{Key: "inittime", Value: -1}}).
			SetSkip(int64(offset)).SetLimit(int64(-limit))
	}

	var coll *mongo.Collection
	if isInResultColl {
		coll = collRouterSwapResult
	} else {
		coll = collRouterSwap
	}

	var cur *mongo.Cursor
	var err error
	switch len(queries) {
	case 0:
		cur, err = coll.Find(clientCtx, bson.M{}, opts)
	case 1:
		cur, err = coll.Find(clientCtx, queries[0], opts)
	default:
		cur, err = coll.Find(clientCtx, bson.M{"$and": queries}, opts)
	}
	if err != nil {
		return nil, mgoError(err)
	}
	result := make([]*MgoSwapResult, 0, 20)
	if isInResultColl {
		err = cur.All(clientCtx, &result)
	} else {
		swaps := make([]*MgoSwap, 0, 20)
		err = cur.All(clientCtx, &swaps)
		if err == nil {
			result = convertToSwapResults(swaps)
		}
	}
	if err != nil {
		return nil, mgoError(err)
	}
	return result, nil
}

// UpdateRouterSwapResult update router swap result
func UpdateRouterSwapResult(fromChainID, txid string, logindex int, items *SwapResultUpdateItems) error {
	key := GetRouterSwapKey(fromChainID, txid, logindex)
	updates := bson.M{
		"timestamp": items.Timestamp,
	}
	if items.Status != KeepStatus {
		updates["status"] = items.Status
	}
	if items.MPC != "" {
		updates["mpc"] = items.MPC
	}
	if items.SwapTx != "" {
		updates["swaptx"] = items.SwapTx
	}
	if items.SwapHeight != 0 {
		updates["swapheight"] = items.SwapHeight
	}
	if items.SwapTime != 0 {
		updates["swaptime"] = items.SwapTime
	}
	if items.SwapValue != "" {
		updates["swapvalue"] = items.SwapValue
	}
	if items.Memo != "" {
		updates["memo"] = items.Memo
	} else if items.Status == MatchTxNotStable {
		updates["memo"] = ""
	}
	if items.SwapNonce != 0 || items.Status == MatchTxNotStable {
		updateResultLock.Lock()
		defer updateResultLock.Unlock()

		err := checkRouterSwapResultUpdate(fromChainID, txid, logindex, items.SwapNonce)
		if err != nil {
			return err
		}
		if items.SwapNonce != 0 {
			updates["swapnonce"] = items.SwapNonce
		}
	}
	_, err := collRouterSwapResult.UpdateByID(clientCtx, key, bson.M{"$set": updates})
	if err == nil {
		log.Info("mongodb update router swap result success", "chainid", fromChainID, "txid", txid, "logindex", logindex, "updates", updates)
	} else {
		log.Error("mongodb update router swap result failed", "chainid", fromChainID, "txid", txid, "logindex", logindex, "updates", updates, "err", err)
	}
	return mgoError(err)
}

func checkRouterSwapResultUpdate(fromChainID, txid string, logindex int, swapnonce uint64) error {
	swapRes, err := FindRouterSwapResult(fromChainID, txid, logindex)
	if err != nil {
		return err
	}

	if swapRes.SwapNonce != 0 {
		log.Error("forbid update swap nonce again", "old", swapRes.SwapNonce, "new", swapnonce)
		return ErrForbidUpdateNonce
	}

	if swapRes.SwapTx != "" {
		log.Error("forbid update swap tx again", "old", swapRes.SwapTx)
		return ErrForbidUpdateSwapTx
	}
	return nil
}

// AddUsedRValue add used r, if error mean already exist
func AddUsedRValue(pubkey, r string) error {
	key := strings.ToLower(r + ":" + pubkey)
	mr := &MgoUsedRValue{
		Key:       key,
		Timestamp: common.NowMilli(),
	}
	_, err := collUsedRValue.InsertOne(clientCtx, mr)
	switch {
	case err == nil:
		log.Info("mongodb add used r success", "pubkey", pubkey, "r", r)
		return nil
	case mongo.IsDuplicateKeyError(err):
		log.Warn("mongodb add used r failed", "pubkey", pubkey, "r", r, "err", err)
		return ErrItemIsDup
	default:
		result := &MgoUsedRValue{}
		if collUsedRValue.FindOne(clientCtx, bson.M{"_id": key}).Decode(result) == nil {
			log.Warn("mongodb add used r failed", "pubkey", pubkey, "r", r, "err", ErrItemIsDup)
			return ErrItemIsDup
		}

		_, err = collUsedRValue.InsertOne(clientCtx, mr) // retry once
		if err != nil {
			log.Warn("mongodb add used r failed in retry", "pubkey", pubkey, "r", r, "err", err)
		}
		return mgoError(err)
	}
}

// ----------------------------- admin functions -------------------------------------

// RouterAdminPassBigValue pass big value
func RouterAdminPassBigValue(fromChainID, txid string, logIndex int) error {
	swap, err := FindRouterSwap(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}
	if swap.Status != TxWithBigValue {
		return fmt.Errorf("swap status is %v, not big value status %v", swap.Status.String(), TxWithBigValue.String())
	}

	_, err = FindRouterSwapResult(fromChainID, txid, logIndex)
	if err == nil {
		return fmt.Errorf("can not pass big value swap with result exist")
	}
	return UpdateRouterSwapStatus(fromChainID, txid, logIndex, TxNotSwapped, time.Now().Unix(), "")
}

// RouterAdminReswap reswap
func RouterAdminReswap(fromChainID, txid string, logIndex int) error {
	swap, err := FindRouterSwap(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}
	if swap.Status != TxProcessed {
		return fmt.Errorf("swap status is %v, can not reswap", swap.Status.String())
	}

	res, err := FindRouterSwapResult(fromChainID, txid, logIndex)
	if err != nil {
		return err
	}
	if res.Status != MatchTxFailed {
		return fmt.Errorf("swap result status is %v, can not reswap", res.Status.String())
	}

	if res.SwapTx == "" {
		return errors.New("swap without swaptx")
	}

	resBridge := router.GetBridgeByChainID(swap.ToChainID)
	if resBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	txStatus, txHash := getSwapResultsTxStatus(resBridge, res)
	if txStatus != nil && txStatus.BlockHeight > 0 && !txStatus.IsSwapTxOnChainAndFailed() {
		_ = UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, MatchTxNotStable, time.Now().Unix(), "")
		return fmt.Errorf("swap succeed with swaptx %v", txHash)
	}

	nonceSetter, ok := resBridge.(tokens.NonceSetter)
	if ok {
		mpcAddress := res.MPC
		nonce, errf := nonceSetter.GetPoolNonce(mpcAddress, "latest")
		if errf != nil {
			log.Warn("get router mpc nonce failed", "address", mpcAddress)
			return errf
		}
		if nonce <= res.SwapNonce {
			return errors.New("can not retry swap with lower nonce")
		}
	}

	log.Info("[reswap] update status to TxNotSwapped", "chainid", fromChainID, "txid", txid, "logIndex", logIndex, "swaptx", res.SwapTx)

	err = UpdateRouterSwapResultStatus(fromChainID, txid, logIndex, Reswapping, time.Now().Unix(), "")
	if err != nil {
		return err
	}

	return UpdateRouterSwapStatus(fromChainID, txid, logIndex, TxNotSwapped, time.Now().Unix(), "")
}

func getSwapResultsTxStatus(bridge tokens.IBridge, res *MgoSwapResult) (status *tokens.TxStatus, txHash string) {
	var err error
	if status, err = bridge.GetTransactionStatus(res.SwapTx); err == nil {
		return status, res.SwapTx
	}
	for _, tx := range res.OldSwapTxs {
		if status, err = bridge.GetTransactionStatus(tx); err == nil {
			return status, tx
		}
	}
	return nil, ""
}

var defaultGetStatusInfoRegisterFilter = []SwapStatus{
	TxNotStable,    // 0
	TxWithBigValue, // 12
}

var defaultGetStatusInfoResultFilter = []SwapStatus{
	MatchTxEmpty,     // 8
	MatchTxNotStable, // 9
	MatchTxFailed,    // 14
}

// GetStatusInfo get status info
func GetStatusInfo(statuses string) (statusInfo map[string]interface{}, err error) {
	registerStatuses, resultStatuses := getStatusesFromStr(statuses)
	if len(registerStatuses) == 0 && len(resultStatuses) == 0 {
		registerStatuses = defaultGetStatusInfoRegisterFilter
		resultStatuses = defaultGetStatusInfoResultFilter
	}

	var registerInfo, resusltInfo []bson.M

	if len(registerStatuses) > 0 {
		registerInfo, err = getStatusInfo(collRouterSwap, registerStatuses)
		if err != nil {
			return nil, mgoError(err)
		}
	}

	if len(resultStatuses) > 0 {
		resusltInfo, err = getStatusInfo(collRouterSwapResult, resultStatuses)
		if err != nil {
			return nil, mgoError(err)
		}
	}

	statusInfo = make(map[string]interface{}, len(registerInfo)+len(resusltInfo))
	for _, m := range registerInfo {
		statusInfo[fmt.Sprint(m["_id"])] = m["count"]
	}
	for _, m := range resusltInfo {
		statusInfo[fmt.Sprint(m["_id"])] = m["count"]
	}
	return statusInfo, nil
}

func getStatusInfo(coll *mongo.Collection, filterStatuses []SwapStatus) (result []bson.M, err error) {
	pipeOption := []bson.M{
		{"$match": bson.M{"status": bson.M{"$in": filterStatuses}}},
		{"$group": bson.M{"_id": "$status", "count": bson.M{"$sum": 1}}},
	}

	ctx, cancel := context.WithDeadline(clientCtx, time.Now().Add(3*time.Second))
	defer cancel()

	cur, err := coll.Aggregate(ctx, pipeOption)
	if err != nil {
		return nil, err
	}

	result = make([]bson.M, 0, 5)
	err = cur.All(ctx, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ----------------------------- helper functions -------------------------------------

// GetRegisteredRouterSwap get registered router swap
func GetRegisteredRouterSwap(fromChainID, txid string, logIndex int) (oldSwap *MgoSwap, registeredOk bool) {
	oldSwap, err := FindRouterSwap(fromChainID, txid, logIndex)
	if err != nil || oldSwap == nil {
		return nil, false
	}
	if oldSwap.Status.IsRegisteredOk() {
		return oldSwap, true
	}
	oldSwapRes, err := FindRouterSwapResult(fromChainID, txid, logIndex)
	if err == nil && oldSwapRes != nil {
		return oldSwap, true
	}
	return oldSwap, false
}

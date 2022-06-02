package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	mapset "github.com/deckarep/golang-set"
)

const (
	acceptAgree    = "AGREE"
	acceptDisagree = "DISAGREE"
)

var (
	cachedAcceptInfos    = mapset.NewSet()
	maxCachedAcceptInfos = 500

	acceptInfoCh      = make(chan *mpc.SignInfoData, 10)
	maxAcceptRoutines = int64(10)
	curAcceptRoutines = int64(0)

	// for fast mpc
	acceptInfoCh2      = make(chan *mpc.SignInfoData, 10)
	maxAcceptRoutines2 = int64(10)
	curAcceptRoutines2 = int64(0)

	// those errors will be ignored in accepting
	errIdentifierMismatch = errors.New("cross chain bridge identifier mismatch")
	errInitiatorMismatch  = errors.New("initiator mismatch")
	errWrongMsgContext    = errors.New("wrong msg context")
)

func getAcceptInfoCh(isFastMPC bool) chan *mpc.SignInfoData {
	if isFastMPC {
		return acceptInfoCh2
	}
	return acceptInfoCh
}

func getAcceptRoutines(isFastMPC bool) (cur, max *int64) {
	if isFastMPC {
		return &curAcceptRoutines2, &maxAcceptRoutines2
	}
	return &curAcceptRoutines, &maxAcceptRoutines
}

// StartAcceptSignJob accept job
func StartAcceptSignJob() {
	logWorker("accept", "start accept sign job")

	openLeveldb()

	if mpcConfig := mpc.GetMPCConfig(false); mpcConfig != nil {
		go startAcceptProducer(mpcConfig)

		utils.TopWaitGroup.Add(1)
		go startAcceptConsumer(mpcConfig)
	}

	if mpcConfig := mpc.GetMPCConfig(true); mpcConfig != nil {
		go startAcceptProducer(mpcConfig)

		utils.TopWaitGroup.Add(1)
		go startAcceptConsumer(mpcConfig)
	}
}

func startAcceptProducer(mpcConfig *mpc.Config) {
	maxAcceptSignTimeInterval := mpcConfig.MaxAcceptSignTimeInterval
	waitInterval := time.Duration(mpcConfig.GetAcceptListLoopInterval) * time.Second
	retryInterval := time.Duration(mpcConfig.GetAcceptListRetryInterval) * time.Second
	if retryInterval > waitInterval {
		retryInterval = waitInterval
	}

	acceptCh := getAcceptInfoCh(mpcConfig.IsFastMPC)

	i := 0
	for {
		if utils.IsCleanuping() {
			return
		}
		signInfo, err := mpcConfig.GetCurNodeSignInfo(maxAcceptSignTimeInterval)
		if err != nil {
			logWorkerError("accept", "getCurNodeSignInfo failed", err)
			time.Sleep(retryInterval)
			continue
		}
		i++
		if i%7 == 0 {
			logWorker("accept", "getCurNodeSignInfo", "count", len(signInfo))
		}
		for _, info := range signInfo {
			if utils.IsCleanuping() {
				return
			}
			if info == nil { // maybe a mpc RPC problem
				continue
			}
			keyID := info.Key
			if cachedAcceptInfos.Contains(keyID) {
				logWorkerTrace("accept", "ignore cached accept sign info before dispatch", "keyID", keyID)
				continue
			}
			logWorker("accept", "dispatch accept sign info", "keyID", keyID)
			acceptCh <- info // produce
		}
		time.Sleep(waitInterval)
	}
}

func startAcceptConsumer(mpcConfig *mpc.Config) {
	defer func() {
		closeLeveldb()
		utils.TopWaitGroup.Done()
	}()

	cur, max := getAcceptRoutines(mpcConfig.IsFastMPC)
	acceptCh := getAcceptInfoCh(mpcConfig.IsFastMPC)

	for {
		select {
		case <-utils.CleanupChan:
			logWorker("accept", "stop accept sign job")
			return
		case info := <-acceptCh: // consume
			// loop and check, break if free worker exist
			for {
				if atomic.LoadInt64(cur) < *max {
					break
				}
				time.Sleep(1 * time.Second)
			}

			atomic.AddInt64(cur, 1)
			go processAcceptInfo(mpcConfig, info)
		}
	}
}

func checkAndUpdateCachedAcceptInfoMap(keyID string) (ok bool) {
	if cachedAcceptInfos.Contains(keyID) {
		logWorker("accept", "ignore cached accept sign info in process", "keyID", keyID)
		return false
	}
	if cachedAcceptInfos.Cardinality() >= maxCachedAcceptInfos {
		cachedAcceptInfos.Pop()
	}
	cachedAcceptInfos.Add(keyID)
	return true
}

func processAcceptInfo(mpcConfig *mpc.Config, info *mpc.SignInfoData) {
	cur, _ := getAcceptRoutines(mpcConfig.IsFastMPC)
	defer atomic.AddInt64(cur, -1)

	keyID := info.Key
	if !checkAndUpdateCachedAcceptInfoMap(keyID) {
		return
	}
	isProcessed := false
	defer func() {
		if !isProcessed {
			cachedAcceptInfos.Remove(keyID)
		}
	}()

	args, err := verifySignInfo(mpcConfig, info)

	ctx := []interface{}{
		"keyID", keyID,
	}
	if args != nil {
		ctx = append(ctx,
			"identifier", args.Identifier,
			"swapType", args.SwapType.String(),
			"fromChainID", args.FromChainID,
			"toChainID", args.ToChainID,
			"swapID", args.SwapID,
			"logIndex", args.LogIndex,
			"tokenID", args.GetTokenID(),
		)
	}

	isPendingInvalidAccept := mpcConfig.PendingInvalidAccept

	switch {
	case // these maybe accepts of other bridges or routers, always discard them
		errors.Is(err, errWrongMsgContext),
		errors.Is(err, errIdentifierMismatch):
		ctx = append(ctx, "err", err)
		logWorkerTrace("accept", "discard sign", ctx...)
		isProcessed = true
		return
	case // these are situations we can not judge, ignore them or disagree immediately
		errors.Is(err, tokens.ErrTxNotStable),
		errors.Is(err, tokens.ErrTxNotFound),
		tokens.IsRPCQueryOrNotFoundError(err):
		if isPendingInvalidAccept {
			ctx = append(ctx, "err", err)
			logWorkerTrace("accept", "ignore sign", ctx...)
			return
		}
	case // these we are sure are config problem, discard them or disagree immediately
		errors.Is(err, errInitiatorMismatch),
		errors.Is(err, tokens.ErrTxWithWrongContract),
		errors.Is(err, tokens.ErrNoBridgeForChainID):
		if isPendingInvalidAccept {
			ctx = append(ctx, "err", err)
			logWorker("accept", "discard sign", ctx...)
			isProcessed = true
			return
		}
	}

	var aggreeMsgContext []string
	agreeResult := acceptAgree
	if err != nil {
		logWorkerError("accept", "DISAGREE sign", err, ctx...)
		agreeResult = acceptDisagree

		disgreeReason := err.Error()
		if len(disgreeReason) > 1000 {
			disgreeReason = disgreeReason[:1000]
		}
		aggreeMsgContext = append(aggreeMsgContext, disgreeReason)
		ctx = append(ctx, "disgreeReason", disgreeReason)
	}
	ctx = append(ctx, "result", agreeResult)

	res, err := mpcConfig.DoAcceptSign(keyID, agreeResult, info.MsgHash, aggreeMsgContext)
	if err != nil {
		ctx = append(ctx, "rpcResult", res)
		logWorkerError("accept", "accept sign job failed", err, ctx...)
	} else {
		logWorker("accept", "accept sign job finish", ctx...)
		isProcessed = true
	}
}

func verifySignInfo(mpcConfig *mpc.Config, signInfo *mpc.SignInfoData) (*tokens.BuildTxArgs, error) {
	msgHash := signInfo.MsgHash
	msgContext := signInfo.MsgContext
	var args tokens.BuildTxArgs
	err := json.Unmarshal([]byte(msgContext[0]), &args)
	if err != nil {
		return nil, errWrongMsgContext
	}
	switch args.Identifier {
	case params.GetIdentifier():
	default:
		return nil, errIdentifierMismatch
	}
	if !mpcConfig.IsMPCInitiator(signInfo.Account) {
		return nil, errInitiatorMismatch
	}
	logWorker("accept", "verifySignInfo", "keyID", signInfo.Key, "msgHash", msgHash, "msgContext", msgContext)
	if lvldbHandle != nil && args.GetTxNonce() > 0 { // only for eth like chain
		err = CheckAcceptRecord(&args)
		if err != nil {
			return &args, err
		}
	}
	err = rebuildAndVerifyMsgHash(signInfo.Key, msgHash, &args)
	return &args, err
}

func getBridges(fromChainID, toChainID string) (srcBridge, dstBridge tokens.IBridge, err error) {
	srcBridge = router.GetBridgeByChainID(fromChainID)
	dstBridge = router.GetBridgeByChainID(toChainID)
	if srcBridge == nil || dstBridge == nil {
		err = tokens.ErrNoBridgeForChainID
	}
	return
}

func rebuildAndVerifyMsgHash(keyID string, msgHash []string, args *tokens.BuildTxArgs) (err error) {
	if !args.SwapType.IsValidType() {
		return fmt.Errorf("unknown router swap type %d", args.SwapType)
	}
	srcBridge, dstBridge, err := getBridges(args.FromChainID.String(), args.ToChainID.String())
	if err != nil {
		return err
	}

	ctx := []interface{}{
		"keyID", keyID,
		"identifier", args.Identifier,
		"swapType", args.SwapType.String(),
		"fromChainID", args.FromChainID,
		"toChainID", args.ToChainID,
		"swapID", args.SwapID,
		"logIndex", args.LogIndex,
		"tokenID", args.GetTokenID(),
	}

	txid := args.SwapID
	logIndex := args.LogIndex
	verifyArgs := &tokens.VerifyArgs{
		SwapType:      args.SwapType,
		LogIndex:      logIndex,
		AllowUnstable: false,
	}
	swapInfo, err := srcBridge.VerifyTransaction(txid, verifyArgs)
	if err != nil {
		logWorkerError("accept", "verifySignInfo failed", err, ctx...)
		return err
	}
	if !strings.EqualFold(args.Bind, swapInfo.Bind) {
		return fmt.Errorf("bind mismatch: '%v' != '%v'", args.Bind, swapInfo.Bind)
	}
	if args.ToChainID.Cmp(swapInfo.ToChainID) != 0 {
		return fmt.Errorf("toChainID mismatch: '%v' != '%v'", args.ToChainID, swapInfo.ToChainID)
	}

	buildTxArgs := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			SwapInfo:    swapInfo.SwapInfo,
			Identifier:  params.GetIdentifier(),
			SwapID:      swapInfo.Hash,
			SwapType:    swapInfo.SwapType,
			Bind:        swapInfo.Bind,
			LogIndex:    swapInfo.LogIndex,
			FromChainID: swapInfo.FromChainID,
			ToChainID:   swapInfo.ToChainID,
			Reswapping:  args.Reswapping,
		},
		From:        args.From,
		OriginFrom:  swapInfo.From,
		OriginTxTo:  swapInfo.TxTo,
		OriginValue: swapInfo.Value,
		Extra:       args.Extra,
	}
	rawTx, err := dstBridge.BuildRawTransaction(buildTxArgs)
	if err != nil {
		logWorkerError("accept", "build raw tx failed", err, ctx...)
		return err
	}
	err = dstBridge.VerifyMsgHash(rawTx, msgHash)
	if err != nil {
		logWorkerError("accept", "verify message hash failed", err, ctx...)
		return err
	}
	logWorker("accept", "verify message hash success", ctx...)
	if lvldbHandle != nil && args.GetTxNonce() > 0 { // only for eth like chain
		go saveAcceptRecord(dstBridge, keyID, buildTxArgs, rawTx, ctx)
	}
	return nil
}

func saveAcceptRecord(bridge tokens.IBridge, keyID string, args *tokens.BuildTxArgs, rawTx interface{}, ctx []interface{}) {
	impl, ok := bridge.(interface {
		GetSignedTxHashOfKeyID(sender, keyID string, rawTx interface{}) (txHash string, err error)
	})
	if !ok {
		return
	}

	swapTx, err := impl.GetSignedTxHashOfKeyID(args.From, keyID, rawTx)
	if err != nil {
		logWorkerError("accept", "get signed tx hash failed", err, ctx...)
		return
	}
	ctx = append(ctx, "swaptx", swapTx)

	err = AddAcceptRecord(args, swapTx)
	if err != nil {
		logWorkerError("accept", "save accept record to db failed", err, ctx...)
		return
	}
	logWorker("accept", "save accept record to db success", ctx...)
}

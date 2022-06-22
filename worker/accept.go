package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tools/fifo"
	mapset "github.com/deckarep/golang-set"
)

const (
	acceptAgree    = "AGREE"
	acceptDisagree = "DISAGREE"
)

type acceptWorkerInfo struct {
	workerCount        int
	acceptInfoQueue    *fifo.Queue // element is signInfo
	acceptInfosInQueue mapset.Set  // element is keyID
}

var (
	cachedAcceptInfos    = mapset.NewSet()
	maxCachedAcceptInfos = 500

	acceptWorkers = make(map[bool]*acceptWorkerInfo)

	// those errors will be ignored in accepting
	errIdentifierMismatch = errors.New("cross chain bridge identifier mismatch")
	errInitiatorMismatch  = errors.New("initiator mismatch")
	errWrongMsgContext    = errors.New("wrong msg context")
)

// StartAcceptSignJob accept job
func StartAcceptSignJob() {
	logWorker("accept", "start accept sign job")

	openLeveldb()
	defer closeLeveldb()

	if mpcConfig := mpc.GetMPCConfig(false); mpcConfig != nil {
		initAcceptWorkers(false)

		go startAcceptProducer(mpcConfig)

		utils.TopWaitGroup.Add(1)
		go startAcceptConsumer(mpcConfig)
	}

	if mpcConfig := mpc.GetMPCConfig(true); mpcConfig != nil {
		initAcceptWorkers(true)

		go startAcceptProducer(mpcConfig)

		utils.TopWaitGroup.Add(1)
		go startAcceptConsumer(mpcConfig)
	}

	utils.TopWaitGroup.Wait()
}

func initAcceptWorkers(isFastMPC bool) {
	acceptWorkers[isFastMPC] = &acceptWorkerInfo{
		workerCount:        10,
		acceptInfoQueue:    fifo.NewQueue(),
		acceptInfosInQueue: mapset.NewSet(),
	}
}

func startAcceptProducer(mpcConfig *mpc.Config) {
	maxAcceptSignTimeInterval := mpcConfig.MaxAcceptSignTimeInterval
	waitInterval := time.Duration(mpcConfig.GetAcceptListLoopInterval) * time.Second
	retryInterval := time.Duration(mpcConfig.GetAcceptListRetryInterval) * time.Second
	if retryInterval > waitInterval {
		retryInterval = waitInterval
	}

	acceptWorker := acceptWorkers[mpcConfig.IsFastMPC]
	acceptInfoQueue := acceptWorker.acceptInfoQueue
	acceptInfosInQueue := acceptWorker.acceptInfosInQueue

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
		if i%7 == 0 {
			logWorker("accept", "getCurNodeSignInfo", "count", len(signInfo), "queue", acceptInfoQueue.Len())
		}
		i++

		for _, info := range signInfo {
			if utils.IsCleanuping() {
				return
			}
			if info == nil { // maybe a mpc RPC problem
				continue
			}
			keyID := info.Key

			if acceptInfosInQueue.Contains(keyID) {
				logWorkerTrace("accept", "ignore accept sign info in queue", "keyID", keyID)
				continue
			}

			if cachedAcceptInfos.Contains(keyID) {
				logWorkerTrace("accept", "ignore accept sign info in cache", "keyID", keyID)
				continue
			}

			_, err = filterSignInfo(info)
			if err != nil {
				logWorkerTrace("accept", "ignore accept sign info", "keyID", keyID, "msgContext", info.MsgContext, "err", err)
				continue
			}

			logWorker("accept", "dispatch accept sign info", "keyID", keyID, "msgContext", info.MsgContext)
			acceptInfoQueue.Add(info)
			acceptInfosInQueue.Add(keyID)
		}
		time.Sleep(waitInterval)
	}
}

func startAcceptConsumer(mpcConfig *mpc.Config) {
	defer utils.TopWaitGroup.Done()

	acceptWorker := acceptWorkers[mpcConfig.IsFastMPC]
	acceptInfoQueue := acceptWorker.acceptInfoQueue
	acceptInfosInQueue := acceptWorker.acceptInfosInQueue

	wg := new(sync.WaitGroup)
	wg.Add(acceptWorker.workerCount)
	for i := 0; i < acceptWorker.workerCount; i++ {
		go func() {
			defer wg.Done()
			for {
				if utils.IsCleanuping() {
					return
				}

				front := acceptInfoQueue.Next()
				if front == nil {
					time.Sleep(3 * time.Second)
					continue
				}

				info := front.(*mpc.SignInfoData)

				logWorker("accept", "process accept sign info start", "keyID", info.Key)
				err := processAcceptInfo(mpcConfig, info)
				if err == nil {
					logWorker("accept", "process accept sign info finish", "keyID", info.Key)
				} else {
					logWorkerError("accept", "process accept sign info finish", err, "keyID", info.Key)
				}

				acceptInfosInQueue.Remove(info.Key)
			}
		}()
	}
	wg.Wait()
}

func checkAndUpdateCachedAcceptInfoMap(keyID string) (ok bool) {
	if cachedAcceptInfos.Contains(keyID) {
		logWorker("accept", "ignore accept sign info in cache", "keyID", keyID)
		return false
	}
	if cachedAcceptInfos.Cardinality() >= maxCachedAcceptInfos {
		cachedAcceptInfos.Pop()
	}
	cachedAcceptInfos.Add(keyID)
	return true
}

func processAcceptInfo(mpcConfig *mpc.Config, info *mpc.SignInfoData) error {
	keyID := info.Key
	if !checkAndUpdateCachedAcceptInfoMap(keyID) {
		return nil
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
		return err
	case // these are situations we can not judge, ignore them or disagree immediately
		errors.Is(err, tokens.ErrTxNotStable),
		errors.Is(err, tokens.ErrTxNotFound),
		tokens.IsRPCQueryOrNotFoundError(err):
		if isPendingInvalidAccept {
			ctx = append(ctx, "err", err)
			logWorkerTrace("accept", "ignore sign", ctx...)
			return err
		}
	case // these we are sure are config problem, discard them or disagree immediately
		errors.Is(err, errInitiatorMismatch),
		errors.Is(err, tokens.ErrTxWithWrongContract),
		errors.Is(err, tokens.ErrNoBridgeForChainID):
		if isPendingInvalidAccept {
			ctx = append(ctx, "err", err)
			logWorker("accept", "discard sign", ctx...)
			isProcessed = true
			return err
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

	logWorker("accept", "accept sign start", "keyID", keyID, "result", agreeResult)
	res, err := mpcConfig.DoAcceptSign(keyID, agreeResult, info.MsgHash, aggreeMsgContext)
	if err != nil {
		ctx = append(ctx, "rpcResult", res)
		logWorkerError("accept", "accept sign failed", err, ctx...)
	} else {
		logWorker("accept", "accept sign finish", ctx...)
		isProcessed = true
	}
	return err
}

func filterSignInfo(signInfo *mpc.SignInfoData) (*tokens.BuildTxArgs, error) {
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
	return &args, err
}

func verifySignInfo(mpcConfig *mpc.Config, signInfo *mpc.SignInfoData) (*tokens.BuildTxArgs, error) {
	args, err := filterSignInfo(signInfo)
	if err != nil {
		return args, err
	}
	if !mpcConfig.IsMPCInitiator(signInfo.Account) {
		return nil, errInitiatorMismatch
	}
	if lvldbHandle != nil && args.GetTxNonce() > 0 { // only for eth like chain
		err = CheckAcceptRecord(args)
		if err != nil {
			return args, err
		}
	}
	err = rebuildAndVerifyMsgHash(signInfo.Key, signInfo.MsgHash, args)
	return args, err
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

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
		start := time.Now()
		signInfo, err := mpcConfig.GetCurNodeSignInfo(maxAcceptSignTimeInterval)
		if err != nil {
			logWorkerError("accept", "getCurNodeSignInfo failed", err, "timespent", time.Since(start).String())
			time.Sleep(retryInterval)
			continue
		}
		if i%7 == 0 {
			logWorker("accept", "getCurNodeSignInfo", "count", len(signInfo), "queue", acceptInfoQueue.Len(), "timespent", time.Since(start).String())
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
					time.Sleep(1 * time.Second)
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
		errors.Is(err, errIdentifierMismatch),
		errors.Is(err, errInvalidAggregate):
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
			logWorker("accept", "ignore sign", ctx...)
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

		disagreeReason := err.Error()
		if len(disagreeReason) > 1000 {
			disagreeReason = disagreeReason[:1000]
		}
		aggreeMsgContext = append(aggreeMsgContext, disagreeReason)
		ctx = append(ctx, "disagreeReason", disagreeReason)
	}
	ctx = append(ctx, "result", agreeResult)

	start := time.Now()
	res, err := mpcConfig.DoAcceptSign(keyID, agreeResult, info.MsgHash, aggreeMsgContext)
	logWorker("accept", "call acceptSign finished", "keyID", keyID, "result", agreeResult, "timespent", time.Since(start).String())
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
	case tokens.AggregateIdentifier:
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
	if args.Identifier == tokens.AggregateIdentifier {
		if err = verifyAggregate(signInfo.MsgHash, args); err != nil {
			logWorkerError("accept", "verify aggregate failed", err, "args", args, "keyID", signInfo.Key)
			return nil, errInvalidAggregate
		}
		return args, nil
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

	start := time.Now()

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
	logWorker("accept", fmt.Sprintf("verifySignInfo success (timespent %v)", time.Since(start).String()), ctx...)
	if !strings.EqualFold(args.Bind, swapInfo.Bind) {
		return fmt.Errorf("bind mismatch: '%v' != '%v'", args.Bind, swapInfo.Bind)
	}
	if args.ToChainID.Cmp(swapInfo.ToChainID) != 0 {
		return fmt.Errorf("toChainID mismatch: '%v' != '%v'", args.ToChainID, swapInfo.ToChainID)
	}

	verifySwapInfo := swapInfo.SwapInfo
	argsSwapInfo := args.SwapArgs.SwapInfo
	if verifySwapInfo.AnyCallSwapInfo != nil &&
		argsSwapInfo.AnyCallSwapInfo != nil &&
		len(argsSwapInfo.AnyCallSwapInfo.Attestation) > 0 {
		verifySwapInfo.AnyCallSwapInfo.Attestation = argsSwapInfo.AnyCallSwapInfo.Attestation
	}

	start = time.Now()
	buildTxArgs := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			SwapInfo:    verifySwapInfo,
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
		logWorkerError("accept", fmt.Sprintf("build raw tx failed (timespent %v)", time.Since(start).String()), err, ctx...)
		return err
	}
	err = dstBridge.VerifyMsgHash(rawTx, msgHash)
	if err != nil {
		logWorkerError("accept", fmt.Sprintf("verify message hash failed (timespent %v)", time.Since(start).String()), err, ctx...)
		return err
	}
	logWorker("accept", fmt.Sprintf("build raw tx and verify message hash success (timespent %v)", time.Since(start).String()), ctx...)
	return nil
}

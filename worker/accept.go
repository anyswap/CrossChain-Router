package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	mapset "github.com/deckarep/golang-set"
)

var (
	cachedAcceptInfos    = mapset.NewSet()
	maxCachedAcceptInfos = 500

	retryInterval = 1 * time.Second
	waitInterval  = 3 * time.Second

	acceptInfoCh      = make(chan *mpc.SignInfoData, 10)
	maxAcceptRoutines = int64(10)
	curAcceptRoutines = int64(0)

	// those errors will be ignored in accepting
	errIdentifierMismatch = errors.New("cross chain bridge identifier mismatch")
	errInitiatorMismatch  = errors.New("initiator mismatch")
	errWrongMsgContext    = errors.New("wrong msg context")
)

// StartAcceptSignJob accept job
func StartAcceptSignJob() {
	logWorker("accept", "start accept sign job")
	go startAcceptProducer()

	utils.TopWaitGroup.Add(1)
	go startAcceptConsumer()
}

func startAcceptProducer() {
	i := 0
	for {
		signInfo, err := mpc.GetCurNodeSignInfo()
		if err != nil {
			logWorkerError("accept", "getCurNodeSignInfo failed", err)
			time.Sleep(retryInterval)
			continue
		}
		i++
		if i%20 == 0 {
			logWorker("accept", "getCurNodeSignInfo", "count", len(signInfo))
		}
		for _, info := range signInfo {
			keyID := info.Key
			if keyID == "" || info.Account == "" || info.GroupID == "" {
				logWorkerWarn("accept", "invalid accept sign info", "signInfo", info)
				continue
			}
			if cachedAcceptInfos.Contains(keyID) {
				logWorkerTrace("accept", "ignore cached accept sign info before dispatch", "keyID", keyID)
				continue
			}
			logWorker("accept", "dispatch accept sign info", "keyID", keyID)
			acceptInfoCh <- info // produce
		}
		time.Sleep(waitInterval)
	}
}

func startAcceptConsumer() {
	defer utils.TopWaitGroup.Done()
	for {
		select {
		case <-utils.CleanupChan:
			logWorker("accept", "stop accept sign job")
			return
		case info := <-acceptInfoCh: // consume
			// loop and check, break if free worker exist
			for {
				if atomic.LoadInt64(&curAcceptRoutines) < maxAcceptRoutines {
					break
				}
				time.Sleep(1 * time.Second)
			}

			atomic.AddInt64(&curAcceptRoutines, 1)
			go processAcceptInfo(info)
		}
	}
}

func checkAndUpdateCachedAcceptInfoMap(keyID string) (ok bool) {
	if cachedAcceptInfos.Contains(keyID) {
		logWorkerTrace("accept", "ignore cached accept sign info in process", "keyID", keyID)
		return false
	}
	if cachedAcceptInfos.Cardinality() >= maxCachedAcceptInfos {
		cachedAcceptInfos.Pop()
	}
	cachedAcceptInfos.Add(keyID)
	return true
}

func processAcceptInfo(info *mpc.SignInfoData) {
	defer atomic.AddInt64(&curAcceptRoutines, -1)

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

	agreeResult := "AGREE"
	args, err := verifySignInfo(info)
	switch {
	case errors.Is(err, tokens.ErrTxNotStable),
		errors.Is(err, tokens.ErrTxNotFound):
		logWorkerTrace("accept", "ignore sign", "keyID", keyID, "err", err)
		return
	case errors.Is(err, errIdentifierMismatch),
		errors.Is(err, errInitiatorMismatch),
		errors.Is(err, errWrongMsgContext),
		errors.Is(err, tokens.ErrTxWithWrongContract),
		errors.Is(err, tokens.ErrNoBridgeForChainID):
		logWorker("accept", "ignore sign", "keyID", keyID, "err", err)
		isProcessed = true
		return
	}
	if err != nil {
		agreeResult = "DISAGREE"
	}
	logWorker("accept", "mpc DoAcceptSign", "keyID", keyID, "result", agreeResult, "chainID", args.FromChainID, "swapID", args.SwapID, "logIndex", args.LogIndex)
	res, err := mpc.DoAcceptSign(keyID, agreeResult, info.MsgHash, info.MsgContext)
	if err != nil {
		logWorkerError("accept", "accept sign job failed", err, "keyID", keyID, "result", res, agreeResult, "chainID", args.FromChainID, "swapID", args.SwapID, "logIndex", args.LogIndex)
	} else {
		logWorker("accept", "accept sign job finish", "keyID", keyID, "result", agreeResult, "chainID", args.FromChainID, "swapID", args.SwapID, "logIndex", args.LogIndex)
		isProcessed = true
	}
}

func verifySignInfo(signInfo *mpc.SignInfoData) (*tokens.BuildTxArgs, error) {
	if !params.IsMPCInitiator(signInfo.Account) {
		return nil, errInitiatorMismatch
	}
	msgHash := signInfo.MsgHash
	msgContext := signInfo.MsgContext
	if len(msgContext) != 1 {
		return nil, errWrongMsgContext
	}
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
	logWorker("accept", "verifySignInfo", "keyID", signInfo.Key, "msgHash", msgHash, "msgContext", msgContext)
	err = rebuildAndVerifyMsgHash(msgHash, &args)
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

func rebuildAndVerifyMsgHash(msgHash []string, args *tokens.BuildTxArgs) (err error) {
	var srcBridge, dstBridge tokens.IBridge
	switch args.SwapType {
	case tokens.RouterSwapType, tokens.AnyCallSwapType:
		srcBridge, dstBridge, err = getBridges(args.FromChainID.String(), args.ToChainID.String())
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown router swap type %v", args.SwapType)
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
		logWorkerError("accept", "verifySignInfo failed", err, "fromChainID", args.FromChainID, "txid", txid, "logIndex", logIndex)
		return err
	}

	buildTxArgs := &tokens.BuildTxArgs{
		SwapArgs:    args.SwapArgs,
		From:        dstBridge.GetChainConfig().GetRouterMPC(),
		OriginValue: swapInfo.Value,
		Extra:       args.Extra,
	}
	rawTx, err := dstBridge.BuildRawTransaction(buildTxArgs)
	if err != nil {
		return err
	}
	return dstBridge.VerifyMsgHash(rawTx, msgHash)
}

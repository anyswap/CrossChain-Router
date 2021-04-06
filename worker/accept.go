package worker

import (
	"container/ring"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/mpc"
	"github.com/anyswap/CrossChain-Router/params"
	"github.com/anyswap/CrossChain-Router/router"
	"github.com/anyswap/CrossChain-Router/tokens"
)

var (
	acceptRing        *ring.Ring
	acceptRingLock    sync.RWMutex
	acceptRingMaxSize = 500

	retryInterval = 3 * time.Second
	waitInterval  = 20 * time.Second

	// those errors will be ignored in accepting
	errIdentifierMismatch = errors.New("cross chain bridge identifier mismatch")
	errInitiatorMismatch  = errors.New("initiator mismatch")
	errWrongMsgContext    = errors.New("wrong msg context")
)

// StartAcceptSignJob accept job
func StartAcceptSignJob() {
	if !params.IsMPCEnabled() {
		logWorker("accept", "no need to start accept sign job as mpc is disabled")
		return
	}
	logWorker("accept", "start accept sign job")
	for {
		signInfo, err := mpc.GetCurNodeSignInfo()
		if err != nil {
			logWorkerError("accept", "getCurNodeSignInfo failed", err)
			time.Sleep(retryInterval)
			continue
		}
		logWorker("accept", "acceptSign", "count", len(signInfo))
		for _, info := range signInfo {
			keyID := info.Key
			history := getAcceptSignHistory(keyID)
			if history != nil && history.result != "IGNORE" {
				logWorker("accept", "history sign", "keyID", keyID, "result", history.result)
				_, _ = mpc.DoAcceptSign(keyID, history.result, history.msgHash, history.msgContext)
				continue
			}
			agreeResult := "AGREE"
			err := verifySignInfo(info)
			switch err {
			case tokens.ErrTxNotStable, tokens.ErrTxNotFound:
				logWorkerTrace("accept", "ignore sign", "keyID", keyID, "err", err)
				continue
			case errIdentifierMismatch,
				errInitiatorMismatch,
				errWrongMsgContext,
				tokens.ErrTxWithWrongContract:
				logWorkerTrace("accept", "ignore sign", "keyID", keyID, "err", err)
				addAcceptSignHistory(keyID, "IGNORE", info.MsgHash, info.MsgContext)
				continue
			}
			if err != nil {
				logWorkerError("accept", "DISAGREE sign", err, "keyID", keyID)
				agreeResult = "DISAGREE"
			}
			logWorker("accept", "mpc DoAcceptSign", "keyID", keyID, "result", agreeResult)
			res, err := mpc.DoAcceptSign(keyID, agreeResult, info.MsgHash, info.MsgContext)
			if err != nil {
				logWorkerError("accept", "accept sign job failed", err, "keyID", keyID, "result", res)
			} else {
				logWorker("accept", "accept sign job finish", "keyID", keyID, "result", agreeResult)
				addAcceptSignHistory(keyID, agreeResult, info.MsgHash, info.MsgContext)
			}
		}
		time.Sleep(waitInterval)
	}
}

func verifySignInfo(signInfo *mpc.SignInfoData) error {
	if !params.IsMPCInitiator(signInfo.Account) {
		return errInitiatorMismatch
	}
	msgHash := signInfo.MsgHash
	msgContext := signInfo.MsgContext
	if len(msgContext) != 1 {
		return errWrongMsgContext
	}
	var args tokens.BuildTxArgs
	err := json.Unmarshal([]byte(msgContext[0]), &args)
	if err != nil {
		return errWrongMsgContext
	}
	switch args.Identifier {
	case params.GetIdentifier():
	case tokens.ReplaceSwapIdentifier:
	default:
		return errIdentifierMismatch
	}
	logWorker("accept", "verifySignInfo", "msgHash", msgHash, "msgContext", msgContext)
	return rebuildAndVerifyMsgHash(msgHash, &args)
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
	case tokens.RouterSwapType:
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

type acceptSignInfo struct {
	keyID      string
	result     string
	msgHash    []string
	msgContext []string
}

func addAcceptSignHistory(keyID, result string, msgHash, msgContext []string) {
	// Create the new item as its own ring
	item := ring.New(1)
	item.Value = &acceptSignInfo{
		keyID:      keyID,
		result:     result,
		msgHash:    msgHash,
		msgContext: msgContext,
	}

	acceptRingLock.Lock()
	defer acceptRingLock.Unlock()

	if acceptRing == nil {
		acceptRing = item
	} else {
		if acceptRing.Len() == acceptRingMaxSize {
			// Drop the block out of the ring
			acceptRing = acceptRing.Move(-1)
			acceptRing.Unlink(1)
			acceptRing = acceptRing.Move(1)
		}
		acceptRing.Move(-1).Link(item)
	}
}

func getAcceptSignHistory(keyID string) *acceptSignInfo {
	acceptRingLock.RLock()
	defer acceptRingLock.RUnlock()

	if acceptRing == nil {
		return nil
	}

	r := acceptRing
	for i := 0; i < r.Len(); i++ {
		item := r.Value.(*acceptSignInfo)
		if item.keyID == keyID {
			return item
		}
		r = r.Prev()
	}

	return nil
}

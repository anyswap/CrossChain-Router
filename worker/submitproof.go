package worker

import (
	"errors"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	mapset "github.com/deckarep/golang-set"
)

var (
	defSubmitProofInterval = int64(300) // seconds
	defMaxSubmitProofTimes = 3

	proofIDQueue = mapset.NewSet()

	proofChan = make(chan *mongodb.MgoSwapResult, 1000)
)

// StartSubmitProofJob submit proof job
func StartSubmitProofJob() {
	logWorker("submitproof", "start submit proof job")

	submitters := params.GetProofSubmitters()
	if len(submitters) == 0 {
		logWorker("submitproof", "stop as no proof submitters exist")
		return
	}

	mongodb.MgoWaitGroup.Add(len(submitters))
	for idx := range submitters {
		go startSubmitProofConsumer(idx)
	}

	go startSubmitProofProducer(mongodb.ProofPrepared)
	go startSubmitProofProducer(mongodb.MatchTxNotStable)
}

func startSubmitProofProducer(status mongodb.SwapStatus) {
	for {
		septime := getSepTimeInFind(maxSubmitProofLifetime)
		res, err := mongodb.FindRouterSwapResultsWithStatus(status, septime)
		if err != nil {
			logWorkerError("submitproof", "find proofs error", err)
		}
		if len(res) > 0 {
			logWorker("submitproof", "find proofs to submit", "count", len(res))
		}
		for _, swap := range res {
			if utils.IsCleanuping() {
				return
			}
			if proofIDQueue.Contains(swap.ProofID) {
				logWorkerTrace("submitproof", "ignore proofID queue", "proofID", swap.ProofID, "chainid", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
				continue
			}
			err = dispatchProof(swap)
			if err != nil {
				logWorkerError("submitproof", "submit proof failed", err, "chainid", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "proofID", swap.ProofID)
			}
		}
		if utils.IsCleanuping() {
			return
		}
		restInJob(restIntervalInSubmitProofJob)
	}
}

func startSubmitProofConsumer(submitterIndex int) {
	defer mongodb.MgoWaitGroup.Done()
	for {
		if utils.IsCleanuping() {
			logWorker("submitproof", "stop submit proof job", "submitter", submitterIndex)
			return
		}

		select {
		case swap := <-proofChan:
			go func() {
				err := submitProof(submitterIndex, swap)
				if err != nil {
					logWorkerError("submitproof", "submit proof failed", err, "submitter", submitterIndex, "chainid", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "proofID", swap.ProofID)
				}
				proofIDQueue.Remove(swap.ProofID)
			}()
		default:
			sleepSeconds(3)
		}
	}
}

func dispatchProof(swap *mongodb.MgoSwapResult) (err error) {
	if swap.ProofID == "" || swap.Proof == "" {
		return errors.New("invlaid proof")
	}
	if swap.SwapHeight > 0 ||
		swap.SpecState == mongodb.ProofConsumed {
		return nil
	}

	if swap.SwapTx != "" {
		maxSubmitProofTimes := params.GetMaxSubmitProofTimes(swap.ToChainID)
		if maxSubmitProofTimes == 0 {
			maxSubmitProofTimes = defMaxSubmitProofTimes
		}
		if len(swap.OldSwapTxs) > maxSubmitProofTimes {
			logWorkerTrace("submitproof", "reached max submit proof times", "maxtimes", maxSubmitProofTimes, "txid", swap.TxID, "logIndex", swap.LogIndex, "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "proofID", swap.ProofID)
			return nil
		}

		submitProofInterval := params.GetSubmitProofInterval(swap.ToChainID)
		if submitProofInterval == 0 {
			submitProofInterval = defSubmitProofInterval
		}
		if getSepTimeInFind(submitProofInterval) < swap.Timestamp {
			logWorkerTrace("submitproof", "wait submit proof interval", "interval", submitProofInterval, "timestamp", swap.Timestamp, "txid", swap.TxID, "logIndex", swap.LogIndex, "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "proofID", swap.ProofID)
			return nil
		}
	}

	proofIDQueue.Add(swap.ProofID)
	proofChan <- swap

	logWorker("submitproof", "dispatch proof", "txid", swap.TxID, "logIndex", swap.LogIndex, "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID, "proofID", swap.ProofID)

	return nil
}

func submitProof(submitterIndex int, swap *mongodb.MgoSwapResult) (err error) {
	fromChainID := swap.FromChainID
	toChainID := swap.ToChainID
	txid := swap.TxID
	logIndex := swap.LogIndex

	resBridge := router.GetBridgeByChainID(toChainID)
	if resBridge == nil {
		logWorkerWarn("submitproof", "bridge not exist", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex)
		return nil
	}

	biFromChainID, biToChainID, biValue, err := getFromToChainIDAndValue(fromChainID, toChainID, swap.Value)
	if err != nil {
		return err
	}

	args := &tokens.BuildTxArgs{
		SwapArgs: tokens.SwapArgs{
			Identifier:  params.GetIdentifier(),
			SwapID:      txid,
			SwapType:    tokens.SwapType(swap.SwapType),
			Bind:        swap.Bind,
			LogIndex:    swap.LogIndex,
			FromChainID: biFromChainID,
			ToChainID:   biToChainID,
			Reswapping:  swap.Status == mongodb.Reswapping,
		},
		SignerIndex: submitterIndex,
		OriginFrom:  swap.From,
		OriginTxTo:  swap.TxTo,
		OriginValue: biValue,
		Extra:       &tokens.AllExtras{},
	}
	args.SwapInfo, err = mongodb.ConvertFromSwapInfo(&swap.SwapInfo)
	if err != nil {
		return err
	}
	signedTx, txHash, err := resBridge.SubmitProof(swap.ProofID, swap.Proof, args)
	if err != nil {
		if errors.Is(err, tokens.ErrProofConsumed) {
			_ = mongodb.MarkProofAsConsumed(fromChainID, txid, logIndex)
		}
		return err
	}

	swapTxNonce := args.GetTxNonce()
	updates := &mongodb.SwapResultUpdateItems{
		SwapTx:    txHash,
		SwapNonce: swapTxNonce,
		SwapValue: args.SwapValue.String(),
		Signer:    args.From,
		Timestamp: now(),
	}
	err = mongodb.UpdateProofSwapResult(fromChainID, txid, logIndex, updates)
	if err != nil {
		logWorkerError("submitproof", "update db status failed", err, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "signer", args.From, "nonce", swapTxNonce)
		return err
	}

	logWorker("submitproof", "submit proof success", "submitter", submitterIndex, "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "txHash", txHash, "signer", args.From, "nonce", swapTxNonce)

	start := time.Now()
	sentTxHash, err := sendSignedTransaction(resBridge, signedTx, args)
	if err == nil && txHash != sentTxHash {
		logWorkerError("submitproof", "send tx success but with different hash", errSendTxWithDiffHash,
			"fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex,
			"txHash", txHash, "sentTxHash", sentTxHash,
			"timespent", time.Since(start).String())
		_ = mongodb.UpdateRouterOldSwapTxs(fromChainID, txid, logIndex, sentTxHash)
	} else if err == nil {
		logWorker("submitproof", "send tx success",
			"fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex,
			"txHash", txHash, "timespent", time.Since(start).String())
	}
	return nil
}

package worker

import (
	"errors"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
)

var (
	defSubmitProofInterval = int64(300) // seconds
	defMaxSubmitProofTimes = 3
)

// StartSubmitProofJob submit proof job
func StartSubmitProofJob() {
	logWorker("submitproof", "start submit proof job")

	mongodb.MgoWaitGroup.Add(2)
	go doSubmitProofJob(mongodb.ProofPrepared)
	go doSubmitProofJob(mongodb.MatchTxNotStable)
}

func doSubmitProofJob(status mongodb.SwapStatus) {
	defer mongodb.MgoWaitGroup.Done()
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
				logWorker("submitproof", "stop submit proof job")
				return
			}
			err = submitProof(swap)
			if err != nil {
				logWorkerError("submitproof", "submit proof failed", err, "chainid", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex, "proofID", swap.ProofID)
			}
		}
		if utils.IsCleanuping() {
			logWorker("submitproof", "stop submit proof job")
			return
		}
		restInJob(restIntervalInSubmitProofJob)
	}
}

func submitProof(swap *mongodb.MgoSwapResult) (err error) {
	if swap.ProofID == "" || swap.Proof == "" {
		return errors.New("invlaid proof")
	}
	if swap.SpecState == mongodb.ProofConsumed {
		return nil
	}

	if swap.SwapTx != "" {
		maxSubmitProofTimes := params.GetMaxSubmitProofTimes(swap.ToChainID)
		if maxSubmitProofTimes == 0 {
			maxSubmitProofTimes = defMaxSubmitProofTimes
		}
		if len(swap.OldSwapTxs) > maxSubmitProofTimes {
			logWorkerTrace("submitproof", "reached max submit proof times", "maxtimes", maxSubmitProofTimes, "txid", swap.TxID, "logIndex", swap.LogIndex, "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID)
			return nil
		}

		submitProofInterval := params.GetSubmitProofInterval(swap.ToChainID)
		if submitProofInterval == 0 {
			submitProofInterval = defSubmitProofInterval
		}
		if getSepTimeInFind(submitProofInterval) < swap.Timestamp {
			logWorkerTrace("submitproof", "wait submit proof interval", "interval", submitProofInterval, "timestamp", swap.Timestamp, "txid", swap.TxID, "logIndex", swap.LogIndex, "fromChainID", swap.FromChainID, "toChainID", swap.ToChainID)
			return nil
		}
	}

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

	logWorker("submitproof", "submit proof success", "fromChainID", fromChainID, "toChainID", toChainID, "txid", txid, "logIndex", logIndex, "txHash", txHash, "signer", args.From, "nonce", swapTxNonce)

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

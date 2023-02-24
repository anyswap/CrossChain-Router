package worker

import (
	"errors"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/cmd/utils"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mongodb"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/cardano"
)

var (
	aggInterval = 7 * 24 * time.Hour

	errInvalidAggregate = errors.New("invalid agregate")
)

func verifyAggregate(msgHash []string, args *tokens.BuildTxArgs) error {
	if args.FromChainID == nil || args.ToChainID == nil ||
		args.FromChainID.Cmp(args.ToChainID) != 0 {
		return errors.New("aggregate: from and to chainid is not same")
	}
	if cardano.BridgeInstance == nil ||
		!cardano.SupportsChainID(args.ToChainID) {
		return errors.New("aggregate: dest chain does not support it")
	}
	return cardano.BridgeInstance.VerifyAggregate(msgHash, args)
}

// StartAggregateJob aggregate job
func StartAggregateJob() {
	logWorker("aggregate", "start router aggregate job")
	if cardano.BridgeInstance == nil {
		return
	}
	mongodb.MgoWaitGroup.Add(1)
	go DoAggregateJob()
	log.Warnf("StartAggregateJob end:%+v", time.Now())
}

func DoAggregateJob() {
	defer mongodb.MgoWaitGroup.Done()
	for {
		if utils.IsCleanuping() {
			return
		}
		logWorker("aggregate", "start aggregate job")
		doAggregateJob()
		logWorker("aggregate", "finish aggregate job")
		time.Sleep(aggInterval)
	}

}

func doAggregateJob() {
	if utils.IsCleanuping() {
		return
	}
	if txHash, err := cardano.BridgeInstance.AggregateTx(); err != nil {
		logWorkerError("aggregate", "aggregate tx err", err)
	} else {
		logWorker("aggregate", "aggregate tx success txHash:", txHash)
	}
}

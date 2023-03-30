package worker

import (
	"time"

	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router/bridge"
)

const interval = 10 * time.Millisecond

// StartRouterSwapWork start router swap job
func StartRouterSwapWork(isServer bool) {
	logWorker("worker", "start router swap worker")

	bridge.InitRouterBridges(isServer)
	bridge.StartReloadRouterConfigTask()

	if !isServer {
		go StartAcceptSignJob()
		time.Sleep(interval)
		StartReportStatJob()
		return
	}

	StartSwapJob()
	time.Sleep(interval)

	StartVerifyJob()
	time.Sleep(interval)

	StartStableJob()
	time.Sleep(interval)

	if params.UseProofSign() {
		StartSubmitProofJob()
		return
	}

	StartReplaceJob()
	time.Sleep(interval)

	StartPassBigValueJob()
	time.Sleep(interval)

	//StartAggregateJob()
	//time.Sleep(interval)

	StartCheckFailedSwapJob()
	time.Sleep(interval)

	StartReswapJob()
	time.Sleep(interval)
}

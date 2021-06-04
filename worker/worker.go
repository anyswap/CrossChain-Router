package worker

import (
	"time"

	"github.com/anyswap/CrossChain-Router/cmd/utils"
	"github.com/anyswap/CrossChain-Router/router/bridge"
	"github.com/anyswap/CrossChain-Router/rpc/client"
)

const interval = 10 * time.Millisecond

// StartRouterSwapWork start router swap job
func StartRouterSwapWork(isServer bool) {
	utils.TopWaitGroup.Add(1)
	defer utils.TopWaitGroup.Done()

	logWorker("worker", "start router swap worker")

	client.InitHTTPClient()
	bridge.InitRouterBridges(isServer)

	if !isServer {
		StartAcceptSignJob()
		return
	}

	StartSwapJob()
	time.Sleep(interval)

	go StartVerifyJob()
	time.Sleep(interval)

	go StartStableJob()
	time.Sleep(interval)

	go StartReplaceJob()
}

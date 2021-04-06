package worker

import (
	"time"

	"github.com/anyswap/CrossChain-Router/router/bridge"
	"github.com/anyswap/CrossChain-Router/rpc/client"
)

const interval = 10 * time.Millisecond

// StartRouterSwapWork start router swap job
func StartRouterSwapWork(isServer bool) {
	logWorker("worker", "start router swap worker")

	client.InitHTTPClient()
	bridge.InitRouterBridges(isServer)

	if !isServer {
		go StartAcceptSignJob()
		return
	}

	go StartVerifyJob()
	time.Sleep(interval)

	go StartSwapJob()
	time.Sleep(interval)

	go StartStableJob()
	time.Sleep(interval)

	go StartReplaceJob()
}

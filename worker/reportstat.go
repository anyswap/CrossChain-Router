package worker

import (
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/rpc/client"
)

var (
	reportStatStarter sync.Once

	reportInterval = 120 * time.Second
)

// StartReportStatJob report stat job
func StartReportStatJob() {
	if params.GetRouterOracleConfig() == nil {
		return
	}
	reportStatStarter.Do(func() {
		logWorker("reportstat", "start report stat job")
		go reportStat()
	})
}

func reportStat() {
	for {
		doReport()

		time.Sleep(reportInterval)
	}
}

func doReport() {
	method := "swap.ReportOracleInfo"
	timestamp := time.Now().Unix()
	args := map[string]interface{}{
		"enode":     mpc.GetSelfEnode(),
		"timestamp": timestamp,
	}
	url := params.GetRouterOracleConfig().ServerAPIAddress
	var result string
	var err error
	for i := 0; i < 3; i++ {
		err = client.RPCPostWithTimeout(20, &result, url, method, args)
		if err == nil {
			break
		}
	}
	if err != nil {
		logWorkerWarn("reportstat", "report stat failed", "err", err)
	} else {
		logWorker("reportstat", "report stat success", "timestamp", timestamp)
	}
}

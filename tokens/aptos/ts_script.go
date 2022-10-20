package aptos

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

func CurrentCallerDir() string {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Dir(file)
	}
	return ""
}

func InstallTsModules() {
	common.MustRunBashCommand(CurrentCallerDir(), "yarn -i")
}

func RunTxHashScript(txbody, argTypes *string, chainId uint) (string, error) {
	cmd := fmt.Sprintf("yarn txhash '%s' '%s' '%d'", *txbody, *argTypes, chainId)
	stats := common.MustRunBashCommand(CurrentCallerDir(), cmd)
	if len(stats) < 2 {
		return "", fmt.Errorf("CalcTxHashByTSScirpt ts output error")
	}
	if !strings.HasPrefix(stats[len(stats)-1], "Done") {
		return "", fmt.Errorf(stats[len(stats)-1])
	}
	return stats[len(stats)-2], nil
}

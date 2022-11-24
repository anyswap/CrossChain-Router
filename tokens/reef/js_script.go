package reef

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/anyswap/CrossChain-Router/v3/common"
)

var script_path = ""

func CurrentCallerDir() string {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Dir(file)
	}
	return ""
}

func InstallJSModules(path string) {
	if len(path) == 0 {
		script_path = CurrentCallerDir()
	} else {
		script_path = path
	}
	common.MustRunBashCommand(script_path, "npm -i")
}

func Public2address(algorithmType, publicKey string) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	cmd := fmt.Sprintf("node public2address '%s' '%s'", algorithmType, publicKey)
	stats := common.MustRunBashCommand(script_path, cmd)
	if len(stats) < 2 {
		return "", fmt.Errorf("Public2address ts output error")
	}
	return stats[0], nil
}

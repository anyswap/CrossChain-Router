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

func GetSignInfo(rawTx, evmAddress, substrateAddress, toAddr string) ([]string, error) {
	if len(script_path) == 0 {
		return nil, fmt.Errorf("script not init")
	}
	cmd := fmt.Sprintf("node public2address %s %s %s %s", rawTx, evmAddress, substrateAddress, toAddr)
	stats := common.MustRunBashCommand(script_path, cmd)
	if len(stats) != 5 {
		return nil, fmt.Errorf("Public2address ts output error")
	}
	return stats, nil
}

func BuildSigningMessage(params []interface{}) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	if params == nil || len(params) != 9 {
		return "", fmt.Errorf("BuildSigningMessage param len dismatch")
	}
	cmd := fmt.Sprintf("node buildRawtx %s %s %s %s %s %s %s %s %s", params...)
	stats := common.MustRunBashCommand(script_path, cmd)
	if len(stats) != 1 {
		return "", fmt.Errorf("buildRawtx ts output error")
	}
	return stats[0], nil
}

func GetTxHash(params []interface{}) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	if params == nil || len(params) != 10 {
		return "", fmt.Errorf("getTxHash param len dismatch")
	}
	cmd := fmt.Sprintf("node getTxHash %s %s %s %s %s %s %s %s %s %s", params...)
	stats := common.MustRunBashCommand(script_path, cmd)
	if len(stats) != 1 {
		return "", fmt.Errorf("getTxHash ts output error")
	}
	return stats[0], nil
}

func SendSignedTx(params []interface{}) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	if params == nil || len(params) != 10 {
		return "", fmt.Errorf("SendSignedTx param len dismatch")
	}
	cmd := fmt.Sprintf("node SendSignedTx %s %s %s %s %s %s %s %s %s %s", params...)
	stats := common.MustRunBashCommand(script_path, cmd)
	if len(stats) != 1 {
		return "", fmt.Errorf("SendSignedTx ts output error")
	}
	return stats[0], nil
}

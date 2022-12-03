package reef

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

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
	common.MustRunBashCommand(script_path, "yarn")
}

func Public2address(algorithmType, publicKey string) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	cmd := fmt.Sprintf("yarn public2address '%s' '%s'", algorithmType, publicKey)
	output := common.MustRunBashCommand(script_path, cmd)
	if len(output) <= 0 {
		return "", fmt.Errorf("Public2address ts output error")
	}
	result := strings.Split(output[len(output)-2], " ")
	if len(result) != 2 {
		return "", fmt.Errorf("Public2address ts output error")
	}
	return result[1], nil
}

func GetSignInfo(rawTx, evmAddress, substrateAddress, toAddr string) ([]string, error) {
	if len(script_path) == 0 {
		return nil, fmt.Errorf("script not init")
	}
	cmd := fmt.Sprintf("yarn getSignInfo %s %s %s %s", rawTx, evmAddress, substrateAddress, toAddr)
	output := common.MustRunBashCommand(script_path, cmd)
	if len(output) <= 0 {
		return nil, fmt.Errorf("getSignInfo ts output error")
	}
	result := strings.Split(output[len(output)-2], " ")
	if len(result) != 5 {
		return nil, fmt.Errorf("getSignInfo ts output error")
	}
	return result, nil
}

func BuildSigningMessage(params []interface{}) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	if params == nil || len(params) != 9 {
		return "", fmt.Errorf("BuildSigningMessage param len dismatch")
	}
	cmd := fmt.Sprintf("yarn buildRawtx %s %s %s %s %s %s %s %s %s", params...)
	output := common.MustRunBashCommand(script_path, cmd)
	if len(output) <= 0 {
		return "", fmt.Errorf("getSignInfo ts output error")
	}
	result := strings.Split(output[len(output)-2], " ")
	if len(result) != 1 {
		return "", fmt.Errorf("buildRawtx ts output error")
	}
	return result[0], nil
}

func SignMessageWithPrivate(params []interface{}) ([]string, error) {
	if len(script_path) == 0 {
		return nil, fmt.Errorf("script not init")
	}
	if params == nil || len(params) != 9 {
		return nil, fmt.Errorf("signTxWallet param len dismatch")
	}
	cmd := fmt.Sprintf("yarn signTxWallet %s %s %s %s %s %s %s %s %s", params...)
	output := common.MustRunBashCommand(script_path, cmd)
	if len(output) <= 0 {
		return nil, fmt.Errorf("signTxWallet ts output error")
	}
	result := strings.Split(output[len(output)-2], " ")
	if len(result) != 2 {
		return nil, fmt.Errorf("signTxWallet ts output error")
	}
	return result, nil
}

func GetTxHash(params []interface{}) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	if params == nil || len(params) != 10 {
		return "", fmt.Errorf("getTxHash param len dismatch")
	}
	cmd := fmt.Sprintf("yarn getTxHash %s %s %s %s %s %s %s %s %s %s", params...)
	output := common.MustRunBashCommand(script_path, cmd)
	if len(output) <= 0 {
		return "", fmt.Errorf("getSignInfo ts output error")
	}
	result := strings.Split(output[len(output)-2], " ")
	if len(result) != 1 {
		return "", fmt.Errorf("getTxHash ts output error")
	}
	return result[0], nil
}

func SendSignedTx(params []interface{}) (string, error) {
	if len(script_path) == 0 {
		return "", fmt.Errorf("script not init")
	}
	if params == nil || len(params) != 10 {
		return "", fmt.Errorf("SendSignedTx param len dismatch")
	}
	cmd := fmt.Sprintf("yarn sendSignedTx %s %s %s %s %s %s %s %s %s %s", params...)
	output := common.MustRunBashCommand(script_path, cmd)
	if len(output) <= 0 {
		return "", fmt.Errorf("getSignInfo ts output error")
	}
	result := strings.Split(output[len(output)-2], " ")
	if len(result) != 1 {
		return "", fmt.Errorf("SendSignedTx ts output error")
	}
	return result[0], nil
}
